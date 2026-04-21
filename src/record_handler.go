package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lib/pq"
)

const (
	// PostgreSQL error codes
	pqErrCodeUniqueViolation = "23505"
    // Intentionally high limit to support long user descriptions and multilingual content
    descriptionMaxLenght = 10000
)

type RecordHandler struct {
	recordRepo   RecordRepository
	categoryRepo CategoryRepository
	adminRepo    AdminRepository
	rorNameCache *RorNameCache
	rorClient    *RorClient
	emailService *EmailService
}

func NewRecordHandlerWithRor(recordRepo RecordRepository, categoryRepo CategoryRepository, adminRepo AdminRepository,
	rorNameCache *RorNameCache, rorClient *RorClient) *RecordHandler {
	return &RecordHandler{
		recordRepo:   recordRepo,
		categoryRepo: categoryRepo,
		adminRepo:    adminRepo,
		rorNameCache: rorNameCache,
		rorClient:    rorClient,
		emailService: NewEmailService(),
	}
}

// CreateRecord handles POST /api/v1/records - Create a new record
func (h *RecordHandler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var user *User
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	user_name, _ := sessionManager.Get(ctx, "name").(string)
	if okO {
		user = &User{
			Name:  user_name,
			Orcid: orcid,
		}
	}

    // Limit teh request body size to 32MB
    r.Body = http.MaxBytesReader(w, r.Body, 32 << 20)

	// Parse the multipart form with a maximum memory of 10 MB for file parts.
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		http.Error(w, "Error parsing multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Retrieve the file part.
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

    filename := strings.TrimSpace(header.Filename)
    filename = filepath.Base(filename)
    infoLogger.Printf("uploaded filename: %s", filename)
    if (filename == "" || filename[0] == '.') {
        errorLogger.Printf("Invalid file: %s. Empty filename or hidden files are not allowed.", filename)
        http.Error(w, "Error: invalid file. Hidden are not allowed", http.StatusBadRequest)
		return
	}
    if strings.Count(filename, ".") != 1 {
        errorLogger.Printf("Invalid file: %s. Multiple extensions are not allowed", filename)
        http.Error(w, "Error: invalid file extension. Multiple extensions are not allowed", http.StatusBadRequest)
		return
	}
    if strings.ToLower(filepath.Ext(filename)) != ".eln" {
        errorLogger.Printf("Invalid file extension: %s", filename)
        http.Error(w, "Error: invalid file extension", http.StatusBadRequest)
		return
	}
    for _, r := range filename {
        if (r <= 0x1F || r == 0x7F || r == '/' || r == '\\') {
            errorLogger.Printf("Invalid file name: %q. Some characters are not allowed.", filename)
            http.Error(w, "Error: invalid file name. Some characters are not allowed.", http.StatusBadRequest)
		    return
        }
    }
    dangerousChars := "<>:\"|?*"
    if strings.ContainsAny(filename, dangerousChars) {
        errorLogger.Printf("Invalid file name: %q. Some characters are not allowed.", filename)
        http.Error(w, "Error: invalid file name. Some characters are not allowed.", http.StatusBadRequest)
		return
    }

	maxBytes := app.MaxFileSize * 1024 * 1024
	if header.Size > maxBytes {
		http.Error(w, fmt.Sprintf("File too large. Maximum allowed is %d MB", app.MaxFileSize), http.StatusRequestEntityTooLarge)
		return
	}

	// assign id
	id, err := getUuidv7()
	if err != nil {
		http.Error(w, "Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

	// 1) Read the first 4 bytes to check for ZIP magic
	sig := make([]byte, 4)
	if _, err := file.Read(sig); err != nil {
		http.Error(w, "could not read file header", http.StatusBadRequest)
		return
	}
	// rewind so later code (hash/upload) sees the whole file
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// 2) Validate ZIP magic
	if !bytes.Equal(sig, []byte{'P', 'K', 0x03, 0x04}) {
		http.Error(w, "uploaded file is not an ELN archive", http.StatusBadRequest)
		return
	}

	hashHex, key, err := hashAndKey(file)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	meta, err := extractRoCrateMetadata(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if len(name) == 0 {
		http.Error(w, "name must be at least one character", http.StatusBadRequest)
		return
	}

	// Parse ROR IDs (optional, can be multiple)
	rorIdsParam := r.FormValue("rors")
	var rorIds []string
	if rorIdsParam != "" {
		// Split by comma and validate each ROR ID
		rawRorIds := strings.Split(rorIdsParam, ",")
		for _, rawRorId := range rawRorIds {
			normalizedRorId, isValid := validateAndNormalizeRorId(strings.TrimSpace(rawRorId))
			if !isValid {
				http.Error(w, fmt.Sprintf("Invalid ROR ID format: %s. Expected format: 0abcdef12 or https://ror.org/0abcdef12", rawRorId), http.StatusBadRequest)
				return
			}
			if normalizedRorId != "" {
				rorIds = append(rorIds, normalizedRorId)
			}
		}
	}

	// Parse category IDs (optional, can be multiple)
	categoriesParam := r.FormValue("categories")
	var categoryIDs []int64
	if categoriesParam != "" {
		// Split by comma and parse each category ID
		categoryIDStrs := strings.Split(categoriesParam, ",")
		for _, categoryIDStr := range categoryIDStrs {
			categoryIDStr = strings.TrimSpace(categoryIDStr)
			if categoryIDStr == "" {
				continue
			}
			categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid category ID: %s", categoryIDStr), http.StatusBadRequest)
				return
			}
			categoryIDs = append(categoryIDs, categoryID)
		}
	}

    description := r.FormValue("description")
	if len(description) > descriptionMaxLenght {
        http.Error(w, fmt.Sprintf(`Description error. Too many characters: %d characters max.`, descriptionMaxLenght), http.StatusBadRequest)
		return
	}

	record := Record{
		Id:            id,
		Sha256:        hashHex,
		Name:          name,
        Description:   description,
		Metadata:      meta,
		UploaderName:  user.Name,
		UploaderOrcid: user.Orcid,
		RorIds:        rorIds,
		License:       "CC-BY-4.0", // All new uploads are CC-BY-4.0
	}

	// Start transaction for record and category associations
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error starting transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Create record using repository
	err = h.recordRepo.Create(ctx, tx, &record, key)
	if err != nil {
		// Check if this is a PostgreSQL unique constraint violation
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == pqErrCodeUniqueViolation {
				if strings.Contains(pqErr.Message, "sha256") || strings.Contains(pqErr.Detail, "sha256") {
					http.Error(w, "Error uploading .eln file: This file already exists in the repository.", http.StatusConflict)
					return
				}
				if strings.Contains(pqErr.Message, "name") || strings.Contains(pqErr.Detail, "name") {
					http.Error(w, "Error uploading .eln file: An entry with this name already exists.", http.StatusConflict)
					return
				}
				// Generic duplicate key error
				http.Error(w, "Error uploading .eln file: A duplicate entry already exists.", http.StatusConflict)
				return
			}
		}
		http.Error(w, fmt.Sprintf("Error inserting record in database: %v", err), http.StatusInternalServerError)
		return
	}

	// Insert category associations if categories were selected
	for _, categoryID := range categoryIDs {
		err = h.categoryRepo.AssociateCategoryWithRecord(ctx, tx, record.Id, categoryID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error associating category %d with record: %v", categoryID, err), http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("Error committing transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Add new ROR IDs to the name cache immediately
	if len(rorIds) > 0 && h.rorNameCache != nil {
		h.rorNameCache.AddRorIds(rorIds)
	}

	// S3 Upload
	if err := h.uploadToS3(file, key); err != nil {
		log.Printf("upload error: %v", err)
		http.Error(w, "failed to upload", http.StatusInternalServerError)
		return
	}

	// Send email notification to admins (async, don't block on errors)
	go func() {
		adminEmails, err := h.adminRepo.GetAllEmails(context.Background())
		if err != nil {
			log.Printf("Failed to get admin emails: %v", err)
			return
		}

		if len(adminEmails) > 0 {
			if err := h.emailService.SendNewRecordNotification(adminEmails, &record, siteUrl); err != nil {
				log.Printf("Failed to send email notification: %v", err)
			}
		}
	}()

	// 2) Decide: JSON (API clients) vs. redirect (browser form)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		// After a POST-from-form, redirect to GET /record/{id}
		http.Redirect(w, r,
			fmt.Sprintf("/records/%s", record.Id),
			http.StatusSeeOther,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	infoLogger.Printf("received new file: %s", record.Id)

	// Send a confirmation response back as JSON.
	if err := json.NewEncoder(w).Encode(record); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// GetRecord handles GET /api/v1/record/{id} - Get a specific record
func (h *RecordHandler) GetRecord(w http.ResponseWriter, r *http.Request, id string) {
	record, err := h.recordRepo.GetByID(r.Context(), id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(record); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// GetRecordHTML handles HTML response for records
func (h *RecordHandler) GetRecordHTML(w http.ResponseWriter, r *http.Request, id string) {
	record, err := h.recordRepo.GetByID(r.Context(), id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			errorLogger.Printf("Database error: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>%#v</pre>", record)
}

// GetRecordMetadata handles metadata.json file download
func (h *RecordHandler) GetRecordMetadata(w http.ResponseWriter, r *http.Request, id string) {
	record, err := h.recordRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Block downloads on archived records
	if record.IsArchived() {
		http.Error(w, "This record has been archived and is not available for download", http.StatusForbidden)
		return
	}

	// Create a human-friendly filename using the record name
	sanitizedName := sanitizeFilename(record.Name)
	filename := fmt.Sprintf("%s-metadata.json", sanitizedName)

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	// Write the metadata JSON
	if _, err := w.Write(record.Metadata); err != nil {
		log.Printf("error streaming metadata for %s to client: %v", id, err)
	}
}

// GetRecordZIP handles ZIP file download
func (h *RecordHandler) GetRecordZIP(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Get the record to access both S3 key and name
	record, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("db error fetching record for %s: %v", id, err)
		}
		return
	}

	// Block downloads on archived records
	if record.IsArchived() {
		http.Error(w, "This record has been archived and is not available for download", http.StatusForbidden)
		return
	}

	// Get the S3 key from the repository
	s3Key, err := h.recordRepo.GetS3Key(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("db error fetching s3_key for %s: %v", id, err)
		}
		return
	}

	// Increment download count
	if _, err := h.recordRepo.IncrementDownloadCount(ctx, id); err != nil {
		log.Printf("warning: failed to increment download count for %s: %v", id, err)
		// Don't fail the download if count increment fails
	}

	// Fetch the object from S3
	s3Client, err := newS3Client()
	if err != nil {
		log.Fatalf("failed to configure S3 client: %v", err)
	}
	bucketName := os.Getenv("BUCKET_NAME")
	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		http.Error(w, "Failed to fetch file", http.StatusBadGateway)
		log.Printf("s3 get error for key %s: %v", s3Key, err)
		return
	}
	defer resp.Body.Close()

	// Create a human-friendly filename using the record name
	sanitizedName := sanitizeFilename(record.Name)
	filename := fmt.Sprintf("%s.eln", sanitizedName)

	// Stream it back to the client
	contentType := aws.ToString(resp.ContentType)
	if contentType == "" {
		contentType = "application/vnd.eln+zip"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("error streaming %s to client: %v", id, err)
	}
}

// IncrementDownloadCount handles POST requests to increment download count
func (h *RecordHandler) IncrementDownloadCount(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	newCount, err := h.recordRepo.IncrementDownloadCount(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("error incrementing download count for %s: %v", id, err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"download_count": newCount})
}

// Router handles routing for record endpoints
func (h *RecordHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/records" && r.Method == "POST":
		h.CreateRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && strings.HasSuffix(path, "/download") && r.Method == "POST":
		h.handleIncrementDownload(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && strings.HasSuffix(path, "/unarchive") && r.Method == "POST":
		h.handleUnarchiveRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && r.Method == "GET":
		h.handleGetRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && (r.Method == "PUT" || r.Method == "PATCH" || r.Method == "POST"):
		h.handleUpdateRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && r.Method == "DELETE":
		h.handleArchiveRecord(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleIncrementDownload processes POST requests to increment download count
func (h *RecordHandler) handleIncrementDownload(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	const suffix = "/download"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	id := strings.TrimSuffix(raw, suffix)

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	h.IncrementDownloadCount(w, r, id)
}

// handleGetRecord processes GET requests for individual records
func (h *RecordHandler) handleGetRecord(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)

	// Check for edit endpoint
	if strings.HasSuffix(raw, "/edit") {
		id := strings.TrimSuffix(raw, "/edit")
		if !uuidv7Regex.MatchString(id) {
			http.Error(w, "Invalid id format", http.StatusBadRequest)
			return
		}
		h.GetEditPage(w, r, id)
		return
	}

	ext := filepath.Ext(raw)
	id := strings.TrimSuffix(raw, ext)

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	if ext == ".eln" {
		h.GetRecordZIP(w, r, id)
		return
	}

	if ext == ".json" {
		h.GetRecordMetadata(w, r, id)
		return
	}

	// Handle content negotiation
	accept := r.Header.Get("Accept")
	parts := strings.Split(accept, ",")
	for _, part := range parts {
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch mt {
		case "application/json", "application/ld+json":
			h.GetRecord(w, r, id)
			return
		case "text/html":
			h.GetRecordHTML(w, r, id)
			return
		}
	}

	// Default to JSON
	h.GetRecord(w, r, id)
}

// handleUpdateRecord processes PUT/PATCH/POST requests for individual records
func (h *RecordHandler) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	id := raw

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	// Check for method override (for HTML forms)
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		if r.FormValue("_method") == "DELETE" {
			h.ArchiveRecord(w, r, id)
			return
		}
	}

	h.UpdateRecord(w, r, id)
}

// handleArchiveRecord processes DELETE requests as archive (soft delete)
func (h *RecordHandler) handleArchiveRecord(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	id := raw

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	h.ArchiveRecord(w, r, id)
}

// handleUnarchiveRecord processes POST requests to unarchive a record
func (h *RecordHandler) handleUnarchiveRecord(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	const suffix = "/unarchive"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
	id := strings.TrimSuffix(raw, suffix)

	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	h.UnarchiveRecord(w, r, id)
}

// uploadToS3 handles S3 upload logic
func (h *RecordHandler) uploadToS3(file multipart.File, key string) error {
	// Rewind so the uploader sees the bytes
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("could not rewind file: %w", err)
		}
	} else {
		return fmt.Errorf("cannot rewind upload")
	}

	s3Client, err := newS3Client()
	if err != nil {
		return fmt.Errorf("failed to configure S3 client: %w", err)
	}
	uploader := manager.NewUploader(s3Client)

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		return fmt.Errorf("BUCKET_NAME not set")
	}

	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("application/vnd.eln+zip"),
	})

	return err
}

// hashAndKey reads from body, returns the hex-encoded SHA256 and the S3 key path.
func hashAndKey(body io.Reader) (hashHex, key string, err error) {
	// Read all into memory (ok up to ~100 MB)
	data, err := io.ReadAll(body)
	if err != nil {
		return "", "", err
	}

	// Compute SHA-256
	sum := sha256.Sum256(data)
	hashHex = hex.EncodeToString(sum[:])

	// Build two-level sharded path: blobs/ab/cd/abcdef… .eln
	key = fmt.Sprintf("%s/%s/%s/%s%s",
		s3Prefix,
		hashHex[0:2],
		hashHex[2:4],
		hashHex,
		fileExt,
	)

	return hashHex, key, nil
}

// extractRoCrateMetadata reads f (a zip) and returns the contents of
// "<root-folder>/ro-crate-metadata.json", or an error if not found.
func extractRoCrateMetadata(f multipart.File) ([]byte, error) {
	// 1) Rewind to the beginning
	if seeker, ok := f.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("cannot rewind file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("file is not seekable")
	}

	// 2) Slurp entire zip into memory (OK up to ~100MB)
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading zip data: %w", err)
	}

	// 3) Open it as a zip archive
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	// 4) Find the first-level root folder name
	var root string
	for _, zf := range zr.File {
		parts := strings.SplitN(zf.Name, "/", 2)
		if len(parts) == 2 {
			root = parts[0]
			break
		}
	}
	if root == "" {
		return nil, fmt.Errorf("no root folder found in zip")
	}

	// 5) Look for "<root>/ro-crate-metadata.json"
	target := root + "/ro-crate-metadata.json"
	for _, zf := range zr.File {
		if zf.Name == target {
			rc, err := zf.Open()
			if err != nil {
				return nil, fmt.Errorf("opening %q: %w", target, err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("%q not found in zip", target)
}

// GetRecordPage handles HTML page rendering for individual records
func (h *RecordHandler) GetRecordPage(w http.ResponseWriter, r *http.Request) {
	funcMap := template.FuncMap{
		"toJson": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}
	var pageTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(staticFiles,
		"templates/layout.html",
		"templates/record.html",
	))
	const prefix = "/record/"
	// Grab the id part in the URL
	raw := strings.TrimPrefix(r.URL.Path, prefix)

	// Split into id and extension
	ext := filepath.Ext(raw) // ".eln" or ""
	id := strings.TrimSuffix(raw, ext)

	// validate id (uuidv7)
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if version query parameter is provided
	versionParam := r.URL.Query().Get("version")
	isHistorical := false
	historyVersion := 0
	var record *Record
	var err error

	if versionParam != "" {
		// Parse version number
		version, parseErr := strconv.Atoi(versionParam)
		if parseErr != nil || version < 1 {
			http.Error(w, "Invalid version number", http.StatusBadRequest)
			return
		}

		// Get historical version
		historyRepo := NewPostgresHistoryRepository(db)
		historyRecord, histErr := historyRepo.GetVersion(ctx, id, version)
		if histErr != nil {
			if histErr == ErrRecordNotFound {
				http.NotFound(w, r)
			} else {
				http.Error(w, "Error fetching historical version", http.StatusInternalServerError)
			}
			return
		}

		// Check permissions for non-approved versions
		if historyRecord.ModerationStatus != StatusApproved {
			// Get current user
			orcid, isAuthenticated := sessionManager.Get(ctx, "orcid").(string)
			if !isAuthenticated {
				http.Error(w, "This version is not publicly available", http.StatusForbidden)
				return
			}

			// Check if user is admin or owner
			isAdmin, _ := h.adminRepo.IsAdmin(ctx, orcid)
			isOwner := historyRecord.UploaderOrcid == orcid

			if !isAdmin && !isOwner {
				http.Error(w, "This version is not publicly available", http.StatusForbidden)
				return
			}
		}

		// Convert RecordHistory to Record for display
		record = &Record{
			Id:               historyRecord.RecordId,
			Name:             historyRecord.Name,
            Description:      historyRecord.Description,
			Sha256:           historyRecord.Sha256,
			Metadata:         historyRecord.Metadata,
			CreatedAt:        historyRecord.CreatedAt,
			ModifiedAt:       historyRecord.ModifiedAt,
			UploaderName:     historyRecord.UploaderName,
			UploaderOrcid:    historyRecord.UploaderOrcid,
			DownloadCount:    historyRecord.DownloadCount,
			ModerationStatus: historyRecord.ModerationStatus,
		}
		isHistorical = true
		historyVersion = version
	} else {
		// Get current record
		record, err = h.recordRepo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "Error fetching record", http.StatusInternalServerError)
			}
			return
		}
	}

	// prettify JSON
	record.MetadataPretty = prettyJSON(record.Metadata)

	// Check if current user can edit this record
	ctx = r.Context()
	canEdit := false
	var user *User
	if orcid, ok := sessionManager.Get(ctx, "orcid").(string); ok {
		name, _ := sessionManager.Get(ctx, "name").(string)
		user = &User{
			Name:  name,
			Orcid: orcid,
		}
		// User owns the record or is admin
		if record.UploaderOrcid == orcid {
			canEdit = true
		} else {
			// Check if user is admin
			if isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid); err == nil && isAdmin {
				canEdit = true
			}
		}
	}

	isArchived := record.IsArchived()

	data := RecordPageData{
		App:            app,
		Record:         *record,
		CanEdit:        canEdit && !isHistorical && !isArchived, // Can't edit historical or archived records
		CanArchive:     canEdit,                                 // Same permission as edit (owner or admin)
		IsArchived:     isArchived,
		User:           user,
		CurrentPage:    "",
		IsHistorical:   isHistorical,
		HistoryVersion: historyVersion,
	}

	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// BrowseRecordShort is a lightweight record representation for the browse API
type BrowseRecordShort struct {
	Id            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	UploaderName  string            `json:"uploaderName"`
	UploaderOrcid string            `json:"uploaderOrcid"`
	Categories    []Category        `json:"categories"`
	RorIds        []string          `json:"rorIds"`
	Organizations []RorOrganization `json:"organizations"` // Organization names from ROR cache
	DownloadCount int               `json:"downloadCount"`
	CreatedAt     int64             `json:"createdAt"`
}

// BrowseAPIResponse is the JSON response for the browse API
type BrowseAPIResponse struct {
	Records    []BrowseRecordShort `json:"records"`
	Pagination struct {
		Page       int `json:"page"`
		PageSize   int `json:"pageSize"`
		TotalCount int `json:"totalCount"`
		TotalPages int `json:"totalPages"`
	} `json:"pagination"`
}

// GetBrowseAPI handles GET /browse?short=1 with Accept: application/json
// Returns a lightweight JSON response for ag-grid to consume
func (h *RecordHandler) GetBrowseAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	categoryIDStr := r.URL.Query().Get("category")
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	rorInput := strings.TrimSpace(r.URL.Query().Get("ror"))

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	// Parse sort parameters
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	// Parse filter parameters
	filterName := strings.TrimSpace(r.URL.Query().Get("filterName"))
	filterNameType := r.URL.Query().Get("filterNameType")
	filterAuthor := strings.TrimSpace(r.URL.Query().Get("filterAuthor"))
	filterAuthorType := r.URL.Query().Get("filterAuthorType")
	filterDownloads := r.URL.Query().Get("filterDownloads")
	filterDownloadsType := r.URL.Query().Get("filterDownloadsType")
	filterDownloadsTo := r.URL.Query().Get("filterDownloadsTo")

	// Validate and map sort field to database column
	var orderByClause string
	switch sortBy {
	case "name":
		orderByClause = "name"
	case "uploaderName":
		orderByClause = "uploader_name"
	case "downloadCount":
		orderByClause = "download_count"
	case "createdAt":
		orderByClause = "created_at"
	default:
		// Default sort by created_at
		orderByClause = "created_at"
	}

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc" // Default to descending
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 10 // default
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	var selectedCategoryID int64
	var selectedCategoryIDs []int64
	var records []Record
	var totalCount int
	var err error

	// Parse category ID(s) if provided
	if categoryIDStr != "" {
		categoryIDStrs := strings.Split(categoryIDStr, ",")
		for _, idStr := range categoryIDStrs {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			categoryID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid category ID", http.StatusBadRequest)
				return
			}
			selectedCategoryIDs = append(selectedCategoryIDs, categoryID)
		}
		if len(selectedCategoryIDs) > 0 {
			selectedCategoryID = selectedCategoryIDs[0]
		}
	}

	// Process ROR input
	var rorIDs []string
	var noRorMatch bool
	if rorInput != "" {
		normalizedRorId, isValid := validateAndNormalizeRorId(rorInput)
		if isValid && normalizedRorId != "" {
			rorIDs = []string{normalizedRorId}
		} else if h.rorNameCache != nil {
			matchingOrgs := h.rorNameCache.Search(rorInput)
			if len(matchingOrgs) > 0 {
				rorIDs = make([]string, len(matchingOrgs))
				for i, org := range matchingOrgs {
					rorIDs[i] = org.ID
				}
			} else {
				noRorMatch = true
			}
		} else {
			http.Error(w, "Invalid ROR ID format and name search not available", http.StatusBadRequest)
			return
		}
	}

	// Build filters map
	filters := make(map[string]interface{})
	if filterName != "" {
		filters["name"] = filterName
		filters["nameType"] = filterNameType
	}
	if filterAuthor != "" {
		filters["author"] = filterAuthor
		filters["authorType"] = filterAuthorType
	}
	if filterDownloads != "" {
		if downloads, err := strconv.Atoi(filterDownloads); err == nil {
			filters["downloads"] = downloads
			filters["downloadsType"] = filterDownloadsType
			if filterDownloadsTo != "" {
				if downloadsTo, err := strconv.Atoi(filterDownloadsTo); err == nil {
					filters["downloadsTo"] = downloadsTo
				}
			}
		}
	}

	// Execute query based on filters
	if noRorMatch {
		records = []Record{}
		totalCount = 0
	} else if searchQuery != "" {
		// Check if search query matches any organization names
		var searchRorIDs []string
		if h.rorNameCache != nil {
			matchingOrgs := h.rorNameCache.Search(searchQuery)
			if len(matchingOrgs) > 0 {
				searchRorIDs = make([]string, len(matchingOrgs))
				for i, org := range matchingOrgs {
					searchRorIDs[i] = org.ID
				}
				log.Printf("API search query '%s' matched %d organizations", searchQuery, len(matchingOrgs))
			}
		}

		records, totalCount, err = h.recordRepo.SearchPaginatedWithRorIDs(ctx, searchQuery, selectedCategoryID, searchRorIDs, pageSize, offset, orderByClause, sortOrder, filters)
	} else if len(rorIDs) > 0 {
		records, totalCount, err = h.recordRepo.GetAllByRorIDsPaginated(ctx, rorIDs, pageSize, offset, orderByClause, sortOrder, filters)
	} else if len(selectedCategoryIDs) > 0 {
		records, totalCount, err = h.recordRepo.GetAllByCategoriesPaginated(ctx, selectedCategoryIDs, pageSize, offset, orderByClause, sortOrder, filters)
	} else {
		records, totalCount, err = h.recordRepo.GetAllPaginated(ctx, pageSize, offset, orderByClause, sortOrder, filters)
	}

	if err != nil {
		log.Printf("Error in GetBrowseAPI: %v", err)
		http.Error(w, "Error fetching records", http.StatusInternalServerError)
		return
	}

	// Build lightweight response
	shortRecords := make([]BrowseRecordShort, 0, len(records))
	for _, rec := range records {
		// Get organization names from ROR cache
		organizations := make([]RorOrganization, 0, len(rec.RorIds))
		if h.rorNameCache != nil {
			for _, rorId := range rec.RorIds {
				if name, found := h.rorNameCache.Get(rorId); found {
					organizations = append(organizations, RorOrganization{
						ID:   rorId,
						Name: name,
					})
				} else {
					// Fallback: just use the ID if not in cache
					organizations = append(organizations, RorOrganization{
						ID:   rorId,
						Name: rorId,
					})
				}
			}
		}

		shortRecords = append(shortRecords, BrowseRecordShort{
			Id:            rec.Id,
			Name:          rec.Name,
            Description:   rec.Description,
			UploaderName:  rec.UploaderName,
			UploaderOrcid: rec.UploaderOrcid,
			Categories:    rec.Categories,
			RorIds:        rec.RorIds,
			Organizations: organizations,
			DownloadCount: rec.DownloadCount,
			CreatedAt:     rec.CreatedAt.Unix(),
		})
	}

	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	response := BrowseAPIResponse{
		Records: shortRecords,
	}
	response.Pagination.Page = page
	response.Pagination.PageSize = pageSize
	response.Pagination.TotalCount = totalCount
	response.Pagination.TotalPages = totalPages

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		errorLogger.Printf("failed to write browse API response: %v", err)
	}
}

// GetBrowsePage handles the browse page that lists all records with pagination
func (h *RecordHandler) GetBrowsePage(w http.ResponseWriter, r *http.Request) {
	// Check if this is an API request (short=1 with Accept: application/json)
	if r.URL.Query().Get("short") == "1" {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			h.GetBrowseAPI(w, r)
			return
		}
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"iterate": func(count int) []int {
			var i int
			var items []int
			for i = 0; i < count; i++ {
				items = append(items, i)
			}
			return items
		},
		"contains": func(slice []int64, item int64) bool {
			for _, v := range slice {
				if v == item {
					return true
				}
			}
			return false
		},
		"toJson": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}

	var pageTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(staticFiles,
		"templates/layout.html",
		"templates/browse.html",
	))

	// CATEGORIES
	categories, err := h.categoryRepo.GetAllHierarchical(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	categoryIDStr := r.URL.Query().Get("category")
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	rorInput := strings.TrimSpace(r.URL.Query().Get("ror"))

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	// Parse sort parameters
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	// Validate and map sort field to database column
	var orderByClause string
	switch sortBy {
	case "name":
		orderByClause = "name"
	case "uploaderName":
		orderByClause = "uploader_name"
	case "downloadCount":
		orderByClause = "download_count"
	case "createdAt":
		orderByClause = "created_at"
	default:
		// Default sort by created_at
		orderByClause = "created_at"
	}

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc" // Default to descending
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 10 // default
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	var selectedCategoryID int64
	var selectedCategoryIDs []int64
	var records []Record
	var totalCount int

	// Parse category ID(s) if provided - supports both single and multiple (comma-separated)
	if categoryIDStr != "" {
		categoryIDStrs := strings.Split(categoryIDStr, ",")
		for _, idStr := range categoryIDStrs {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			categoryID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid category ID", http.StatusBadRequest)
				return
			}
			selectedCategoryIDs = append(selectedCategoryIDs, categoryID)
		}
		// Keep selectedCategoryID for backward compatibility (first category)
		if len(selectedCategoryIDs) > 0 {
			selectedCategoryID = selectedCategoryIDs[0]
		}
	}

	// Process ROR input - can be either a ROR ID or organization name
	var rorID string
	var rorIDs []string // Multiple ROR IDs when searching by name
	var rorOrgName string
	var rorSearchInput string // Store the original input for display
	var noRorMatch bool       // Flag to indicate no matching organizations found
	if rorInput != "" {
		rorSearchInput = rorInput // Store original input for display
		// Try to validate as ROR ID first
		normalizedRorId, isValid := validateAndNormalizeRorId(rorInput)
		if isValid && normalizedRorId != "" {
			// It's a valid ROR ID
			rorID = normalizedRorId
			rorIDs = []string{normalizedRorId}
		} else {
			// It's not a valid ROR ID, treat it as organization name search
			// Use ROR name cache to find matching organizations
			if h.rorNameCache != nil {
				matchingOrgs := h.rorNameCache.Search(rorInput)
				if len(matchingOrgs) > 0 {
					// Collect all matching organization ROR IDs
					rorIDs = make([]string, len(matchingOrgs))
					orgNames := make([]string, 0, len(matchingOrgs))
					for i, org := range matchingOrgs {
						rorIDs[i] = org.ID
						orgNames = append(orgNames, org.Name)
					}
					// Use the first matching organization for display
					rorID = matchingOrgs[0].ID
					if len(matchingOrgs) == 1 {
						rorOrgName = matchingOrgs[0].Name
					} else {
						// Multiple matches - show count
						rorOrgName = fmt.Sprintf("%d organizations matching '%s'", len(matchingOrgs), rorInput)
					}
					log.Printf("Found %d ROR organizations matching '%s': %v", len(matchingOrgs), rorInput, orgNames)
				} else {
					// No matching organizations found
					log.Printf("No ROR organizations found matching name: %s", rorInput)
					// Set flag to skip query execution and return empty results
					noRorMatch = true
				}
			} else {
				// Name cache not available, treat as invalid input
				log.Printf("ROR name cache not available, cannot search by organization name")
				http.Error(w, "Invalid ROR ID format and name search not available", http.StatusBadRequest)
				return
			}
		}
	}

	// Determine which query to execute based on search, category, and ROR parameters
	if noRorMatch {
		// No matching ROR organizations found, return empty results
		records = []Record{}
		totalCount = 0
	} else if searchQuery != "" {
		// Check if search query matches any organization names
		var searchRorIDs []string
		if h.rorNameCache != nil {
			matchingOrgs := h.rorNameCache.Search(searchQuery)
			if len(matchingOrgs) > 0 {
				searchRorIDs = make([]string, len(matchingOrgs))
				for i, org := range matchingOrgs {
					searchRorIDs[i] = org.ID
				}
				log.Printf("Search query '%s' matched %d organizations", searchQuery, len(matchingOrgs))
			}
		}

		// Search with optional category filter and organization matches
		records, totalCount, err = h.recordRepo.SearchPaginatedWithRorIDs(r.Context(), searchQuery, selectedCategoryID, searchRorIDs, pageSize, offset, orderByClause, sortOrder, make(map[string]interface{}))
		if err != nil {
			log.Printf("Error in GetBrowsePage searching for '%s': %v", searchQuery, err)
			http.Error(w, "Error searching records", http.StatusInternalServerError)
			return
		}
	} else if len(rorIDs) > 0 {
		// Filter by ROR ID(s) (either directly provided or found via name search)
		// Multiple ROR IDs - use the multi-ID query
		records, totalCount, err = h.recordRepo.GetAllByRorIDsPaginated(r.Context(), rorIDs, pageSize, offset, orderByClause, sortOrder, make(map[string]interface{}))
		if err != nil {
			log.Printf("Error in GetBrowsePage filtering by ROR IDs %v: %v", rorIDs, err)
			http.Error(w, "Error fetching records for ROR organizations", http.StatusInternalServerError)
			return
		}

		// Fetch ROR organization name if not already set
		if rorOrgName == "" && len(rorIDs) == 1 {
			if org, err := h.rorClient.GetOrganization(rorIDs[0]); err == nil {
				rorOrgName = org.Name
			} else {
				log.Printf("Error fetching ROR organization name for %s: %v", rorIDs[0], err)
				rorOrgName = rorIDs[0] // Fallback to ID if fetch fails
			}
		}
	} else if len(selectedCategoryIDs) > 0 {
		// Filter by categories (single or multiple)
		records, totalCount, err = h.recordRepo.GetAllByCategoriesPaginated(r.Context(), selectedCategoryIDs, pageSize, offset, orderByClause, sortOrder, make(map[string]interface{}))
		if err != nil {
			log.Printf("Error in GetBrowsePage filtering by categories %v: %v", selectedCategoryIDs, err)
			http.Error(w, fmt.Sprintf("Error fetching records for categories"), http.StatusInternalServerError)
			return
		}
	} else {
		// Get all records
		records, totalCount, err = h.recordRepo.GetAllPaginated(r.Context(), pageSize, offset, orderByClause, sortOrder, make(map[string]interface{}))
		if err != nil {
			http.Error(w, "Error fetching records", http.StatusInternalServerError)
			return
		}
	}

	recs := make([]Record, 0, len(records))
	for _, r := range records {
		// clone r (shallow copy), then set only MetadataPretty
		r.MetadataPretty = prettyJSON(r.Metadata)
		recs = append(recs, r)
	}

	// Calculate pagination info
	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	// Get current user info
	ctx := r.Context()
	var user *User
	var isAdmin bool
	if orcid, ok := sessionManager.Get(ctx, "orcid").(string); ok {
		name, _ := sessionManager.Get(ctx, "name").(string)
		user = &User{
			Name:  name,
			Orcid: orcid,
		}
		// Check if user is admin
		if adminStatus, err := h.adminRepo.IsAdmin(ctx, orcid); err == nil {
			isAdmin = adminStatus
		}
	}

	// Get style nonce from context (set by browseSecurityHeaders middleware)
	styleNonce := ""
	if nonce, ok := r.Context().Value(nonceContextKey).(string); ok {
		styleNonce = nonce
	}

	data := struct {
		App                 App
		Categories          []Category
		Records             []Record
		SelectedCategoryID  int64
		SelectedCategoryIDs []int64
		SelectedRorID       string
		SelectedRorName     string
		SelectedRorInput    string
		SearchQuery         string
		User                *User
		IsAdmin             bool
		Page                int
		PageSize            int
		TotalCount          int
		TotalPages          int
		CurrentPage         string
		StyleNonce          string
	}{
		App:                 app,
		Categories:          categories,
		Records:             recs,
		SelectedCategoryID:  selectedCategoryID,
		SelectedCategoryIDs: selectedCategoryIDs,
		SelectedRorID:       rorID,
		SelectedRorName:     rorOrgName,
		SelectedRorInput:    rorSearchInput,
		SearchQuery:         searchQuery,
		User:                user,
		IsAdmin:             isAdmin,
		Page:                page,
		PageSize:            pageSize,
		TotalCount:          totalCount,
		TotalPages:          totalPages,
		CurrentPage:         "browse",
		StyleNonce:          styleNonce,
	}

	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", data)
}

// UpdateRecord handles PUT/PATCH requests to update a record
func (h *RecordHandler) UpdateRecord(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get the existing record to check ownership
	existingRecord, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Check if user owns this record or is admin
	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil {
		http.Error(w, "Error checking admin status", http.StatusInternalServerError)
		return
	}

	if existingRecord.UploaderOrcid != orcid && !isAdmin {
		http.Error(w, "You can only edit your own records", http.StatusForbidden)
		return
	}

	// Parse multipart form (for file upload support)
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		log.Printf("DEBUG: Form parsing error: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Check if a new file was uploaded
	file, header, fileErr := r.FormFile("file")
	hasNewFile := fileErr == nil
	if hasNewFile {
		defer file.Close()

		// Validate file size
		maxBytes := app.MaxFileSize * 1024 * 1024
		if header.Size > maxBytes {
			http.Error(w, fmt.Sprintf("File too large. Maximum allowed is %d MB", app.MaxFileSize), http.StatusRequestEntityTooLarge)
			return
		}

		// Validate ZIP magic
		sig := make([]byte, 4)
		if _, err := file.Read(sig); err != nil {
			http.Error(w, "could not read file header", http.StatusBadRequest)
			return
		}
		if seeker, ok := file.(io.Seeker); ok {
			seeker.Seek(0, io.SeekStart)
		}
		if !bytes.Equal(sig, []byte{'P', 'K', 0x03, 0x04}) {
			http.Error(w, "uploaded file is not an ELN archive", http.StatusBadRequest)
			return
		}
	}

	// Get updated values
	name := r.FormValue("name")
	if len(name) == 0 {
		http.Error(w, "name must be at least one character", http.StatusBadRequest)
		return
	}

	// Parse ROR IDs (optional, can be multiple)
	rorIdsParam := r.FormValue("rors")
	var rorIds []string
	if rorIdsParam != "" {
		rawRorIds := strings.Split(rorIdsParam, ",")
		for _, rawRorId := range rawRorIds {
			normalizedRorId, isValid := validateAndNormalizeRorId(strings.TrimSpace(rawRorId))
			if !isValid {
				http.Error(w, fmt.Sprintf("Invalid ROR ID format: %s", rawRorId), http.StatusBadRequest)
				return
			}
			if normalizedRorId != "" {
				rorIds = append(rorIds, normalizedRorId)
			}
		}
	}

	// Parse category IDs (optional, can be multiple)
	categoriesParam := r.FormValue("categories")
	var categoryIDs []int64
	if categoriesParam != "" {
		// Split by comma and parse each category ID
		categoryIDStrs := strings.Split(categoriesParam, ",")
		for _, categoryIDStr := range categoryIDStrs {
			categoryIDStr = strings.TrimSpace(categoryIDStr)
			if categoryIDStr == "" {
				continue
			}
			categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid category ID: %s", categoryIDStr), http.StatusBadRequest)
				return
			}
			categoryIDs = append(categoryIDs, categoryID)
		}
	}

	description := r.FormValue("description")
	if len(description) > descriptionMaxLenght {
        http.Error(w, fmt.Sprintf(`Description error. Too many characters: %d characters max.`, descriptionMaxLenght), http.StatusBadRequest)
		return
	}

	// Update the record
	updatedRecord := *existingRecord
	updatedRecord.Name = name
	updatedRecord.Description = description
	updatedRecord.RorIds = rorIds

	// If new file uploaded, process it
	var newS3Key string
	if hasNewFile {
		// Calculate hash and S3 key
		hashHex, key, err := hashAndKey(file)
		if err != nil {
			http.Error(w, "failed to process file", http.StatusInternalServerError)
			return
		}
		newS3Key = key
		updatedRecord.Sha256 = hashHex

		// Extract metadata
		meta, err := extractRoCrateMetadata(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updatedRecord.Metadata = meta

		// Upload to S3
		if err := h.uploadToS3(file, key); err != nil {
			log.Printf("upload error: %v", err)
			http.Error(w, "failed to upload", http.StatusInternalServerError)
			return
		}
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error starting transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update record
	if hasNewFile {
		// For new file uploads, insert into history as pending version instead of updating main record
		// Get next version number
		var nextVersion int
		err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(version), 0) + 1 FROM record_history WHERE record_id = $1`,
			updatedRecord.Id,
		).Scan(&nextVersion)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting next version: %v", err), http.StatusInternalServerError)
			return
		}

		// Insert new version into history with pending status
		_, err = tx.ExecContext(ctx,
			`INSERT INTO record_history (
				record_id, version, s3_key, name, description, sha256, metadata,
				uploader_name, uploader_orcid, download_count,
				created_at, modified_at, moderation_status, change_type
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, $10, 'pending', 'PENDING_VERSION')`,
			updatedRecord.Id, nextVersion, newS3Key, updatedRecord.Name, updatedRecord.Description, updatedRecord.Sha256, updatedRecord.Metadata,
			existingRecord.UploaderName, existingRecord.UploaderOrcid, existingRecord.CreatedAt,
		)
		if err != nil {
			// Check if this is a PostgreSQL unique constraint violation
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code == pqErrCodeUniqueViolation {
					if strings.Contains(pqErr.Message, "sha256") || strings.Contains(pqErr.Detail, "sha256") {
						http.Error(w, "Error uploading new version: This file already exists in the repository.", http.StatusConflict)
						return
					}
				}
			}
			http.Error(w, fmt.Sprintf("Error inserting pending version: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Update only name or description (no moderation needed for metadata-only updates)
		_, err = tx.ExecContext(ctx,
			`UPDATE records SET name = $2, description = $3, modified_at = now() WHERE id = $1`,
			updatedRecord.Id, updatedRecord.Name, updatedRecord.Description,
		)
		if err != nil {
			// Check if this is a PostgreSQL unique constraint violation on name only
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code == pqErrCodeUniqueViolation {
					if strings.Contains(pqErr.Message, "name") || strings.Contains(pqErr.Detail, "name") {
						http.Error(w, "Error updating entry: An entry with this name already exists.", http.StatusConflict)
						return
					}
				}
			}
			http.Error(w, fmt.Sprintf("Error updating record: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Update ROR associations
	_, err = tx.ExecContext(ctx, `DELETE FROM records_ror WHERE record_id = $1`, updatedRecord.Id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error clearing ROR associations: %v", err), http.StatusInternalServerError)
		return
	}
	rorRepo := NewPostgresRorRepository(db)
	for _, rorId := range updatedRecord.RorIds {
		if err := rorRepo.AssociateRorWithRecord(ctx, tx, updatedRecord.Id, rorId); err != nil {
			http.Error(w, fmt.Sprintf("Error associating ROR: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Clear existing category associations and insert new ones
	// First, we need to clear existing associations
	err = h.categoryRepo.ClearRecordCategories(ctx, tx, updatedRecord.Id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error clearing existing categories: %v", err), http.StatusInternalServerError)
		return
	}

	// Insert new category associations
	for _, categoryID := range categoryIDs {
		err = h.categoryRepo.AssociateCategoryWithRecord(ctx, tx, updatedRecord.Id, categoryID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error associating category %d with record: %v", categoryID, err), http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("Error committing transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Add new ROR IDs to the name cache immediately
	if len(updatedRecord.RorIds) > 0 && h.rorNameCache != nil {
		h.rorNameCache.AddRorIds(updatedRecord.RorIds)
	}

	// Redirect to record page
	http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
}

// ArchiveRecord handles archive (soft delete) requests for a record
func (h *RecordHandler) ArchiveRecord(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get the existing record to check ownership
	existingRecord, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Check if user owns this record or is admin
	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil {
		http.Error(w, "Error checking admin status", http.StatusInternalServerError)
		return
	}

	if existingRecord.UploaderOrcid != orcid && !isAdmin {
		http.Error(w, "You can only archive your own records", http.StatusForbidden)
		return
	}

	// Parse archive reason from form
	reason := strings.TrimSpace(r.FormValue("archive_reason"))
	if reason == "" {
		http.Error(w, "Archive reason is required", http.StatusBadRequest)
		return
	}

	// Archive the record (soft delete)
	err = h.recordRepo.Archive(ctx, id, reason)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error archiving record: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect to record page
	http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
}

// UnarchiveRecord handles unarchive requests for a record
func (h *RecordHandler) UnarchiveRecord(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get the existing record to check ownership
	existingRecord, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Check if user owns this record or is admin
	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil {
		http.Error(w, "Error checking admin status", http.StatusInternalServerError)
		return
	}

	if existingRecord.UploaderOrcid != orcid && !isAdmin {
		http.Error(w, "You can only unarchive your own records", http.StatusForbidden)
		return
	}

	// Unarchive the record
	err = h.recordRepo.Unarchive(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unarchiving record: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect to record page
	http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
}

// GetEditPage handles GET requests for the edit form
func (h *RecordHandler) GetEditPage(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	name, _ := sessionManager.Get(ctx, "name").(string)
	user := &User{
		Name:  name,
		Orcid: orcid,
	}

	// Get the existing record
	record, err := h.recordRepo.GetByID(ctx, id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Check if user owns this record or is admin
	isAdmin, err := h.adminRepo.IsAdmin(ctx, orcid)
	if err != nil {
		http.Error(w, "Error checking admin status", http.StatusInternalServerError)
		return
	}

	if record.UploaderOrcid != orcid && !isAdmin {
		http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
		return
	}

	// Redirect to record page if archived (prevent editing)
	if record.IsArchived() {
		http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
		return
	}

	// Get all categories for the dropdown
	categories, err := h.categoryRepo.GetAllHierarchical(ctx)
	if err != nil {
		http.Error(w, "Error fetching categories", http.StatusInternalServerError)
		return
	}

	// Render edit template
	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/edit.html",
	))

	data := struct {
		App         App
		Record      Record
		Categories  []Category
		User        *User
		CurrentPage string
	}{
		App:         app,
		Record:      *record,
		Categories:  categories,
		User:        user,
		CurrentPage: "",
	}

	w.Header().Set("Content-Type", "text/html")
	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

