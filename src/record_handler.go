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
		RorIds:        rorIds,
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

// Router handles routing for record endpoints
func (h *RecordHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/records" && r.Method == "POST":
		h.CreateRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && r.Method == "GET":
		h.handleGetRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && (r.Method == "PUT" || r.Method == "PATCH" || r.Method == "POST"):
		h.handleUpdateRecord(w, r)
	case strings.HasPrefix(path, "/api/v1/record/") && r.Method == "DELETE":
		h.handleDeleteRecord(w, r)
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
			h.DeleteRecord(w, r, id)
			return
		}
	}

	h.UpdateRecord(w, r, id)
}

// handleDeleteRecord processes DELETE requests for individual records
func (h *RecordHandler) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
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

	h.DeleteRecord(w, r, id)
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

	// Check if current user can edit this record
	ctx := r.Context()
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

	data := RecordPageData{
		App:         app,
		Record:      *record,
		CanEdit:     canEdit,
		User:        user,
		CurrentPage: "",
	}

	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// GetBrowsePage handles the browse page that lists all records with pagination
func (h *RecordHandler) GetBrowsePage(w http.ResponseWriter, r *http.Request) {
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
	}

	var pageTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(staticFiles,
		"templates/layout.html",
		"templates/browse.html",
	))

	// CATEGORIES
	categories, err := h.categoryRepo.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	categoryIDStr := r.URL.Query().Get("category")
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	rorID := strings.TrimSpace(r.URL.Query().Get("ror"))

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

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
	var records []Record
	var totalCount int

	// Parse category ID if provided
	if categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid category ID", http.StatusBadRequest)
			return
		}
		selectedCategoryID = categoryID
	}

	// Determine which query to execute based on search, category, and ROR parameters
	if searchQuery != "" {
		// Search with optional category filter
		records, totalCount, err = h.recordRepo.SearchPaginated(r.Context(), searchQuery, selectedCategoryID, pageSize, offset)
		if err != nil {
			log.Printf("Error in GetBrowsePage searching for '%s': %v", searchQuery, err)
			http.Error(w, "Error searching records", http.StatusInternalServerError)
			return
		}
	} else if rorID != "" {
		// Filter by ROR ID
		records, totalCount, err = h.recordRepo.GetAllByRorIDPaginated(r.Context(), rorID, pageSize, offset)
		if err != nil {
			log.Printf("Error in GetBrowsePage filtering by ROR %s: %v", rorID, err)
			http.Error(w, fmt.Sprintf("Error fetching records for ROR %s", rorID), http.StatusInternalServerError)
			return
		}
	} else if selectedCategoryID > 0 {
		// Filter by category only
		records, totalCount, err = h.recordRepo.GetAllByCategoryPaginated(r.Context(), selectedCategoryID, pageSize, offset)
		if err != nil {
			log.Printf("Error in GetBrowsePage filtering by category %d: %v", selectedCategoryID, err)
			http.Error(w, fmt.Sprintf("Error fetching records for category %d", selectedCategoryID), http.StatusInternalServerError)
			return
		}
	} else {
		// Get all records
		records, totalCount, err = h.recordRepo.GetAllPaginated(r.Context(), pageSize, offset)
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

	// Fetch ROR organization name if filtering by ROR
	var rorOrgName string
	if rorID != "" {
		rorClient := NewRorClient()
		if org, err := rorClient.GetOrganization(rorID); err == nil {
			rorOrgName = org.Name
		} else {
			log.Printf("Error fetching ROR organization name for %s: %v", rorID, err)
			rorOrgName = rorID // Fallback to ID if fetch fails
		}
	}

	data := struct {
		App                App
		Categories         []Category
		Records            []Record
		SelectedCategoryID int64
		SelectedRorID      string
		SelectedRorName    string
		SearchQuery        string
		User               *User
		IsAdmin            bool
		Page               int
		PageSize           int
		TotalCount         int
		TotalPages         int
		CurrentPage        string
	}{
		App:                app,
		Categories:         categories,
		Records:            recs,
		SelectedCategoryID: selectedCategoryID,
		SelectedRorID:      rorID,
		SelectedRorName:    rorOrgName,
		SearchQuery:        searchQuery,
		User:               user,
		IsAdmin:            isAdmin,
		Page:               page,
		PageSize:           pageSize,
		TotalCount:         totalCount,
		TotalPages:         totalPages,
		CurrentPage:        "browse",
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

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("DEBUG: Form parsing error: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
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

	// Update the record
	updatedRecord := *existingRecord
	updatedRecord.Name = name
	updatedRecord.RorIds = rorIds

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error starting transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update record
	err = h.recordRepo.Update(ctx, tx, &updatedRecord)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error updating record: %v", err), http.StatusInternalServerError)
		return
	}

	// Insert category association if a category was selected
	if hasCategory {
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

	// Redirect to record page
	http.Redirect(w, r, fmt.Sprintf("/record/%s", id), http.StatusSeeOther)
}

// DeleteRecord handles DELETE requests to remove a record
func (h *RecordHandler) DeleteRecord(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get the existing record to check ownership and get S3 key
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
		http.Error(w, "You can only delete your own records", http.StatusForbidden)
		return
	}

	// Get S3 key for file deletion
	s3Key, err := h.recordRepo.GetS3Key(ctx, id)
	if err != nil {
		http.Error(w, "Error getting S3 key", http.StatusInternalServerError)
		return
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error starting transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Delete from database
	err = h.recordRepo.Delete(ctx, tx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error deleting record: %v", err), http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("Error committing transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete from S3
	if err := h.deleteFromS3(s3Key); err != nil {
		log.Printf("Warning: Failed to delete S3 object %s: %v", s3Key, err)
		// Don't fail the request if S3 deletion fails
	}

	// Redirect to browse page
	http.Redirect(w, r, "/browse", http.StatusSeeOther)
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

	// Get all categories for the dropdown
	categories, err := h.categoryRepo.GetAll(ctx)
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

// deleteFromS3 handles S3 file deletion
func (h *RecordHandler) deleteFromS3(key string) error {
	s3Client, err := newS3Client()
	if err != nil {
		return fmt.Errorf("failed to configure S3 client: %w", err)
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		return fmt.Errorf("BUCKET_NAME not set")
	}

	_, err = s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})

	return err
}
