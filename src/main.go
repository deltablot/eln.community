/**
 * eln.community
 * © 2025 - Nicolas CARPi, Deltablot
 * License: AGPLv3
 */
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"

	"github.com/google/uuid"
)

//go:generate bash build.sh

type Record struct {
	CreatedAt time.Time       `json:"created_at"`
	Id        string          `json:"id"`
	Metadata  json.RawMessage `json:"metadata"`
	// This will be ignored by json.Marshal
	MetadataPretty string    `json:"-"`
	ModifiedAt     time.Time `json:"modified_at"`
	Name           string    `json:"name"`
	Sha256         string    `json:"sha256"`
	UploaderName   string    `json:"uploader_name"`
	UploaderOrcid  string    `json:"uploader_orcid"`
}

type Category struct {
	Id         int64
	Name       string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type User struct {
	Name  string
	Orcid string
}

//go:embed dist/index.js* dist/main.css* templates/*.html dist/favicon.ico dist/robots.txt
var staticFiles embed.FS

var (
	infoLogger  = log.New(os.Stdout, "[info] ", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "[error] ", log.LstdFlags|log.Lshortfile)
)

var db *sql.DB

var sessionManager *scs.SessionManager

// this will be overwritten during docker build
var version string = "dev"

var maxFileSizeStr = "1024"

var maxFileSize int64

var defaultMaxTotalFiles int64 = 24

var siteUrl = "http://localhost"

// uuidv7Regex ensures that the filename follows the format:
// UUID with version 7 (third group starts with '7')
// For example: "123e4567-e89b-7d89-a456-426614174000"
var uuidv7Regex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-7[a-fA-F0-9]{3}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// used for cache busting of assets
var buildId string

// ensureSchema loads src/sql/structure.sql if the public schema has no tables yet.
func ensureSchema(ctx context.Context) error {
	// 1) Check if any tables exist in public schema
	var tableCount int
	err := db.QueryRowContext(ctx, `
        SELECT COUNT(*)
          FROM information_schema.tables
         WHERE table_schema = 'public'
           AND table_type   = 'BASE TABLE'
    `).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("checking existing tables: %w", err)
	}
	if tableCount > 0 {
		// schema already initialized
		return nil
	}

	// 2) Read the SQL file
	sqlBytes, err := os.ReadFile("src/sql/structure.sql")
	if err != nil {
		return fmt.Errorf("reading structure.sql: %w", err)
	}

	// 3) Execute all statements in the file
	if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("executing structure.sql: %w", err)
	}

	return nil
}

func getUuidv7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	return id.String(), nil
}

const (
	s3Prefix = "records"
	fileExt  = ".eln"
)

func initMaxFileSize() int64 {
	maxFileSizeStr = "1024"
	if os.Getenv("MAX_FILE_SIZE_MB") != "" {
		maxFileSizeStr = os.Getenv("MAX_FILE_SIZE_MB")
	}
	maxFileSize, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
	if err != nil {
		errorLogger.Fatalf("Server misconfiguration: invalid MAX_FILE_SIZE_MB %v", err)
	}
	return maxFileSize
}

func initBuildId() {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("Failed to generate random id: %v", err)
	}
	buildId = hex.EncodeToString(b)
}

// S3 stuff
func newS3Client() (*s3.Client, error) {
	accessKey := os.Getenv("ACCESS_KEY")
	secretKey := os.Getenv("SECRET_KEY")
	region := os.Getenv("REGION")
	if accessKey == "" || secretKey == "" || region == "" {
		log.Fatal("environment variables ACCESS_KEY, SECRET_KEY and REGION must be set")
	}
	// Custom endpoint resolver pointing at Scaleway S3
	endpointResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           "https://s3." + region + ".scw.cloud",
				SigningRegion: region,
			}, nil
		},
	)

	// Load standard config, injecting creds, region, and endpoint override
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithEndpointResolverWithOptions(endpointResolver),
	)
	if err != nil {
		return nil, err
	}

	// Create S3 client; enable path‐style if required
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return client, nil
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

// POST Handler
func postHandler(w http.ResponseWriter, r *http.Request) {
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
	// Retrieve the key from the request header.
	/*
		headerKey := r.Header.Get("Authorization")
		if headerKey != buildId {
			http.Error(w, "Unauthorized: invalid key", http.StatusUnauthorized)
			return
		}
	*/

	// Parse the multipart form with a maximum memory of 10 MB for file parts.
	// Files larger than this size will be stored in temporary files.
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

	maxBytes := maxFileSize * 1024 * 1024
	if header.Size > maxBytes {
		http.Error(w, fmt.Sprintf("File too large. Maximum allowed is %d MB", maxFileSize), http.StatusRequestEntityTooLarge)
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

	record := Record{
		Id:            id,
		Sha256:        hashHex,
		Name:          name,
		Metadata:      meta,
		UploaderName:  user.Name,
		UploaderOrcid: user.Orcid,
	}

	// DB insert
	_, err = db.Exec(
		`INSERT INTO records (id, s3_key, sha256, name, metadata, uploader_name, uploader_orcid) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		record.Id, key, record.Sha256, record.Name, record.Metadata, record.UploaderName, record.UploaderOrcid,
	)
	// Will create an error if sha256 is not unique
	if err != nil {
		http.Error(w, fmt.Sprintf("Error inserting row in database: %v", err), http.StatusInternalServerError)
		return
	}

	// S3
	// Rewind so the uploader sees the bytes
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "could not rewind file", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "cannot rewind upload", http.StatusInternalServerError)
		return
	}

	s3Client, err := newS3Client()
	if err != nil {
		log.Fatalf("failed to configure S3 client: %v", err)
	}
	uploader := manager.NewUploader(s3Client)

	bucketName := os.Getenv("BUCKET_NAME")
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("application/vnd.eln+zip"),
	})

	if err != nil {
		log.Printf("upload error: %v", err)
		http.Error(w, "failed to upload", http.StatusInternalServerError)
		return
	}

	// 2) Decide: JSON (API clients) vs. redirect (browser form)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		// After a POST-from-form, redirect to GET /record/{id}
		// Use 303 See Other so browsers use GET on the new URL
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
func getCategories(ctx context.Context) ([]Category, error) {
	rows, err := db.QueryContext(ctx, `
	  SELECT id, name, created_at, modified_at FROM categories
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Iterate and build slice
	var recs []Category
	for rows.Next() {
		var r Category
		if err := rows.Scan(
			&r.Id,
			&r.Name,
			&r.CreatedAt,
			&r.ModifiedAt,
		); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return recs, nil
}

func scanRecords(ctx context.Context) ([]Record, error) {
	rows, err := db.QueryContext(ctx, `
	  SELECT id, sha256, name, metadata, created_at, modified_at FROM records
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Iterate and build slice
	var recs []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(
			&r.Id,
			&r.Sha256,
			&r.Name,
			&r.Metadata,
			&r.CreatedAt,
			&r.ModifiedAt,
		); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return recs, nil

}

// fetch a Record from db by id
func scanRecord(ctx context.Context, id string) (Record, error) {
	var rec Record
	err := db.QueryRowContext(ctx, `
	    SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid
        FROM records
        WHERE id = $1
        `, id).Scan(
		&rec.Id,
		&rec.Sha256,
		&rec.Name,
		&rec.Metadata,
		&rec.CreatedAt,
		&rec.ModifiedAt,
		&rec.UploaderName,
		&rec.UploaderOrcid,
	)
	if err != nil {
		return Record{}, err
	}
	return rec, nil
}

func getAbout(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFiles(
		"src/templates/layout.html",
		"src/templates/about.html",
	))
	pageTmpl.ExecuteTemplate(w, "layout", nil)
}

func getBrowse(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFiles(
		"src/templates/layout.html",
		"src/templates/browse.html",
	))
	records, err := scanRecords(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	categories, err := getCategories(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}
	var recs []Record
	for _, rec := range records {
		// pretty–print the metadata JSON
		var obj interface{}
		if err := json.Unmarshal(rec.Metadata, &obj); err != nil {
			// fallback to raw bytes
			recs = append(recs, Record{
				CreatedAt:      rec.CreatedAt,
				Id:             rec.Id,
				Metadata:       rec.Metadata,
				MetadataPretty: string(rec.Metadata),
				ModifiedAt:     rec.ModifiedAt,
				Name:           rec.Name,
				Sha256:         rec.Sha256,
				UploaderName:   rec.UploaderName,
				UploaderOrcid:  rec.UploaderOrcid,
			})
		} else {
			b, _ := json.MarshalIndent(obj, "", "  ")
			recs = append(recs, Record{
				CreatedAt:      rec.CreatedAt,
				Id:             rec.Id,
				Metadata:       rec.Metadata,
				MetadataPretty: string(b),
				ModifiedAt:     rec.ModifiedAt,
				Name:           rec.Name,
				Sha256:         rec.Sha256,
				UploaderName:   rec.UploaderName,
				UploaderOrcid:  rec.UploaderOrcid,
			})
		}
	}
	rootData := struct {
		BuildId     string
		Categories  []Category
		Records     []Record
		MaxFileSize int64
		Version     string
	}{
		BuildId:     buildId,
		Categories:  categories,
		Records:     recs,
		MaxFileSize: maxFileSize,
		Version:     version,
	}
	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", rootData)
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFiles(
		"src/templates/layout.html",
		"src/templates/index.html",
	))
	records, err := scanRecords(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	categories, err := getCategories(r.Context())
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}
	var recs []Record
	for _, rec := range records {
		// pretty–print the metadata JSON
		var obj interface{}
		if err := json.Unmarshal(rec.Metadata, &obj); err != nil {
			// fallback to raw bytes
			recs = append(recs, Record{
				Id:             rec.Id,
				Sha256:         rec.Sha256,
				CreatedAt:      rec.CreatedAt,
				ModifiedAt:     rec.ModifiedAt,
				MetadataPretty: string(rec.Metadata),
			})
		} else {
			b, _ := json.MarshalIndent(obj, "", "  ")
			recs = append(recs, Record{
				Id:             rec.Id,
				Sha256:         rec.Sha256,
				CreatedAt:      rec.CreatedAt,
				ModifiedAt:     rec.ModifiedAt,
				MetadataPretty: string(b),
			})
		}
	}

	ctx := r.Context()
	var user *User
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	name, _ := sessionManager.Get(ctx, "name").(string)
	if okO {
		user = &User{
			Name:  name,
			Orcid: orcid,
		}
	}

	rootData := struct {
		BuildId     string
		Categories  []Category
		Records     []Record
		MaxFileSize int64
		Version     string
		User        *User
	}{
		BuildId:     buildId,
		Categories:  categories,
		Records:     recs,
		MaxFileSize: maxFileSize,
		Version:     version,
		User:        user,
	}

	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", rootData)
}

func getRecord(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFiles(
		"src/templates/layout.html",
		"src/templates/record.html",
	))
	const prefix = "/records/"
	// 1) Make sure the path has our prefix
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	// 2) Trim off the prefix, e.g. "0196…c0b1.eln" or just "0196…c0b1"
	raw := strings.TrimPrefix(r.URL.Path, prefix)

	// 3) Split into id and extension
	ext := filepath.Ext(raw) // ".eln" or ""
	id := strings.TrimSuffix(raw, ext)

	// 4) Validate only the UUID part
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}
	rec, err := scanRecord(r.Context(), id)
	if err != nil {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}
	// pretty–print the metadata JSON
	var obj interface{}
	var record Record
	if err := json.Unmarshal(rec.Metadata, &obj); err != nil {
		// fallback to raw bytes
		record = Record{
			Id:             rec.Id,
			Sha256:         rec.Sha256,
			Name:           rec.Name,
			CreatedAt:      rec.CreatedAt,
			ModifiedAt:     rec.ModifiedAt,
			UploaderName:   rec.UploaderName,
			UploaderOrcid:  rec.UploaderOrcid,
			MetadataPretty: string(rec.Metadata),
		}
	} else {
		b, _ := json.MarshalIndent(obj, "", "  ")
		record = Record{
			Id:             rec.Id,
			Sha256:         rec.Sha256,
			Name:           rec.Name,
			CreatedAt:      rec.CreatedAt,
			ModifiedAt:     rec.ModifiedAt,
			UploaderName:   rec.UploaderName,
			UploaderOrcid:  rec.UploaderOrcid,
			MetadataPretty: string(b),
		}
	}
	rootData := struct {
		BuildId string
		Record  Record
		Version string
	}{
		BuildId: buildId,
		Record:  record,
		Version: version,
	}

	//pageTmpl.ExecuteTemplate(w, "layout", rootData)
	if err := pageTmpl.ExecuteTemplate(w, "layout", rootData); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// securityHeaders is a middleware that injects your CSP, HSTS, etc.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://cdn.jsdelivr.net; "+
				"style-src 'self' https://cdn.jsdelivr.net; "+
				"img-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none';"+
				"upgrade-insecure-requests;",
		)
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func getIndexHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := scanRecords(r.Context())
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			errorLogger.Printf("%v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

var (
	oneHandlers = map[string]func(w http.ResponseWriter, r *http.Request, id string){
		"application/json":        handleJSON,
		"application/ld+json":     handleJSON,
		"text/html":               handleHTML,
		"application/vnd.eln+zip": handleZIP,
	}
)

// GET /records
func getFileHandler(w http.ResponseWriter, r *http.Request) {

	const prefix = "/api/v1/records/"
	// 1) Make sure the path has our prefix
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	// 2) Trim off the prefix, e.g. "0196…c0b1.eln" or just "0196…c0b1"
	raw := strings.TrimPrefix(r.URL.Path, prefix)

	// 3) Split into id and extension
	ext := filepath.Ext(raw) // ".eln" or ""
	id := strings.TrimSuffix(raw, ext)

	// 4) Validate only the UUID part
	if !uuidv7Regex.MatchString(id) {
		http.Error(w, "Invalid id format", http.StatusBadRequest)
		return
	}

	if ext == ".eln" {
		handleZIP(w, r, id)
		return
	}

	// 1) Parse the Accept header into individual media types
	accept := r.Header.Get("Accept")
	parts := strings.Split(accept, ",")
	for _, part := range parts {
		// strip any params (e.g. ;q=0.9) and whitespace
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])

		// 2) See if we have a handler for it
		if handler, ok := oneHandlers[mt]; ok {
			handler(w, r, id)
			return
		}
	}
	// 3) If none matched
	http.Error(w, "Not Acceptable", http.StatusNotAcceptable)
}

func handleJSON(w http.ResponseWriter, r *http.Request, id string) {
	record, err := scanRecord(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
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

func handleHTML(w http.ResponseWriter, r *http.Request, id string) {
	record, err := scanRecord(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
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

func handleZIP(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// 1) Get the S3 key from the database
	var s3Key string
	err := db.QueryRowContext(ctx,
		`SELECT s3_key FROM records WHERE id = $1`, id,
	).Scan(&s3Key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Printf("db error fetching s3_key for %s: %v", id, err)
		return
	}

	// 2) Fetch the object from S3
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

	// 3) Stream it back to the client
	//   - Use the Content-Type from S3 if available, else default
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

// serveAsset will pick the .br version if the client accepts it.
func serveAsset(w http.ResponseWriter, r *http.Request) {
	// strip leading slash
	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	// detect mime type
	ext := path.Ext(reqPath)
	w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	// if client supports brotli, try .br
	if strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
		if f, err := staticFiles.Open("dist/" + reqPath + ".br"); err == nil {
			defer f.Close()
			w.Header().Set("Content-Encoding", "br")
			io.Copy(w, f)
			return
		}
	}
	// fallback to uncompressed
	f, err := staticFiles.Open("dist/" + reqPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

func main() {
	infoLogger.Printf("starting eln.community version: %s", version)
	// Define and parse command-line flags.
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	maxFileSize = initMaxFileSize()

	initBuildId()

	// Expect DATABASE_URL like:
	// postgres://user:pass@host:port/dbname?sslmode=disable
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("set DATABASE_URL")
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	// Verify connectivity
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db.Ping: %v", err)
	}
	if err := ensureSchema(ctx); err != nil {
		log.Fatalf("failed to initialize schema: %v", err)
	}

	// 2) Configure SCS
	sessionManager = scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = true // set to true if you're on HTTPS
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	siteUrlEnv := os.Getenv("SITE_URL")
	if len(siteUrlEnv) > 10 {
		siteUrl = siteUrlEnv
	}

	initOIDC(siteUrl)

	addr := ":" + *port
	infoLogger.Printf("server running on port: %s", *port)

	mux := http.NewServeMux()

	// 1) Static & healthcheck
	mux.HandleFunc("/favicon.ico", serveAsset)
	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// OIDC routes
	mux.HandleFunc("/auth/login", loginHandler)
	mux.HandleFunc("/auth/callback", callbackHandler)
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		sessionManager.Destroy(r.Context())
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// 2) API (no CSP middleware)
	mux.HandleFunc("/api/v1/records", postHandler)
	mux.HandleFunc("/api/v1/records/", getFileHandler)

	// 3) About page (with CSP middleware)
	mux.Handle("/about", securityHeaders(http.HandlerFunc(getAbout)))
	mux.Handle("/browse", securityHeaders(http.HandlerFunc(getBrowse)))
	mux.Handle("/records/", securityHeaders(http.HandlerFunc(getRecord)))

	// 4) Home / catch-all HTML (with CSP middleware)
	mux.Handle("/", securityHeaders(http.HandlerFunc(getRoot)))

	// in prod we embed the files, but in dev we serve them directly to avoid having to recompile binary after a change
	if os.Getenv("DEV") == "1" {
		mux.HandleFunc("/index.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "src/index.js")
		})
		mux.HandleFunc("/partage.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "src/partage.js")
		})
		mux.HandleFunc("/utils.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "src/utils.js")
		})
		mux.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "src/main.css")
		})
		infoLogger.Printf("dev service running at: http://localhost:%s", *port)
	} else { // PROD
		http.HandleFunc("GET /index.js", serveAsset)
		http.HandleFunc("GET /robots.txt", serveAsset)
		http.HandleFunc("GET /partage.js", serveAsset)
		http.HandleFunc("GET /utils.js", serveAsset)
		http.HandleFunc("GET /index.css", serveAsset)
		http.HandleFunc("GET /main.css", serveAsset)
		infoLogger.Printf("service running at: %s", siteUrl)
	}

	// Wrap all handlers so they get a request-scoped session context
	handler := sessionManager.LoadAndSave(mux)

	if err := http.ListenAndServe(addr, handler); err != nil {
		errorLogger.Fatalf("failed to start server: %v", err)
	}
}
