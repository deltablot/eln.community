package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

type RecordHandler struct {
	recordRepo   RecordRepository
	categoryRepo CategoryRepository
	adminRepo    AdminRepository
}

func NewRecordHandler(recordRepo RecordRepository, categoryRepo CategoryRepository, adminRepo AdminRepository) *RecordHandler {
	return &RecordHandler{
		recordRepo:   recordRepo,
		categoryRepo: categoryRepo,
		adminRepo:    adminRepo,
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

	// Parse category ID (optional)
	categoryIDStr := r.FormValue("category")
	var categoryID int64
	var hasCategory bool
	if categoryIDStr != "" {
		var err error
		categoryID, err = strconv.ParseInt(categoryIDStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid category ID: %s", categoryIDStr), http.StatusBadRequest)
			return
		}
		hasCategory = true
	}

	record := Record{
		Id:            id,
		Sha256:        hashHex,
		Name:          name,
		Metadata:      meta,
		UploaderName:  user.Name,
		UploaderOrcid: user.Orcid,
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
		http.Error(w, fmt.Sprintf("Error inserting record in database: %v", err), http.StatusInternalServerError)
		return
	}

	// Insert category association if a category was selected
	if hasCategory {
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

	// S3 Upload
	if err := h.uploadToS3(file, key); err != nil {
		log.Printf("upload error: %v", err)
		http.Error(w, "failed to upload", http.StatusInternalServerError)
		return
	}

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

// GetRecordZIP handles ZIP file download
func (h *RecordHandler) GetRecordZIP(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Get the S3 key from the repository
	s3Key, err := h.recordRepo.GetS3Key(ctx, id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("db error fetching s3_key for %s: %v", id, err)
		}
		return
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

	// Stream it back to the client
	contentType := aws.ToString(resp.ContentType)
	if contentType == "" {
		contentType = "application/vnd.eln+zip"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("error streaming %s to client: %v", id, err)
	}
}

// Router handles routing for record endpoints
func (h *RecordHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/records" && r.Method == "POST":
		h.CreateRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && r.Method == "GET":
		h.handleGetRecord(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRecord processes GET requests for individual records
func (h *RecordHandler) handleGetRecord(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/record/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, prefix)
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
	var pageTmpl = template.Must(template.ParseFS(staticFiles,
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

	// get record
	record, err := h.recordRepo.GetByID(r.Context(), id)
	if err != nil {
		if err == ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Error fetching record", http.StatusInternalServerError)
		}
		return
	}

	// prettify JSON
	record.MetadataPretty = prettyJSON(record.Metadata)

	data := RecordPageData{
		App:    app,
		Record: *record,
	}

	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// GetBrowsePage handles the browse page that lists all records
func (h *RecordHandler) GetBrowsePage(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/browse.html",
	))

	// CATEGORIES
	categories, err := h.categoryRepo.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	// Parse category filter parameter
	categoryIDStr := r.URL.Query().Get("category")
	var selectedCategoryID int64
	var records []Record

	if categoryIDStr != "" {
		// Filter by category
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid category ID", http.StatusBadRequest)
			return
		}
		selectedCategoryID = categoryID
		records, err = h.recordRepo.GetAllByCategory(r.Context(), categoryID)
		if err != nil {
			log.Printf("Error in GetBrowsePage filtering by category %d: %v", categoryID, err)
			http.Error(w, fmt.Sprintf("Error fetching records for category %d", categoryID), http.StatusInternalServerError)
			return
		}
	} else {
		// Get all records
		records, err = h.recordRepo.GetAll(r.Context())
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

	data := struct {
		App                App
		Categories         []Category
		Records            []Record
		SelectedCategoryID int64
	}{
		App:                app,
		Categories:         categories,
		Records:            recs,
		SelectedCategoryID: selectedCategoryID,
	}

	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", data)
}
