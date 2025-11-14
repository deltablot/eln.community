/**
 * eln.community
 * © 2025 - Nicolas CARPi, Deltablot
 * License: AGPLv3
 */
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/google/uuid"
)

//go:generate bash build.sh

type Record struct {
	CreatedAt time.Time       `json:"created_at"`
	Id        string          `json:"id"`
	Metadata  json.RawMessage `json:"metadata"`
	// This will be ignored by json.Marshal
	MetadataPretty string     `json:"-"`
	ModifiedAt     time.Time  `json:"modified_at"`
	Name           string     `json:"name"`
	Sha256         string     `json:"sha256"`
	UploaderName   string     `json:"uploader_name"`
	UploaderOrcid  string     `json:"uploader_orcid"`
	RorIds         []string   `json:"rors,omitempty"`
	Categories     []Category `json:"categories,omitempty"`
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

type App struct {
	BuildId     string
	MaxFileSize int64
	Version     string
}

type RootPageData struct {
	App
	Categories []Category
	User       *User
}

type RecordPageData struct {
	App
	Record  Record
	CanEdit bool
	User    *User
}

type RecordsPageData struct {
	App
	Categories []Category
	Records    []Record
	User       *User
	IsAdmin    bool
}

//go:embed dist/index.js* dist/main.css* templates/*.html dist/favicon.ico dist/robots.txt
var staticFiles embed.FS

var (
	infoLogger  = log.New(os.Stdout, "[info] ", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "[error] ", log.LstdFlags|log.Lshortfile)
)

var db *sql.DB

var sessionManager *scs.SessionManager

var app App

// this will be overwritten during docker build
var version string = "dev"

var siteUrl = "http://localhost"

// uuidv7Regex ensures that the filename follows the format:
// UUID with version 7 (third group starts with '7')
// For example: "123e4567-e89b-7d89-a456-426614174000"
var uuidv7Regex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-7[a-fA-F0-9]{3}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// used for cache busting of assets
var buildId string

// ensureSchema runs migrations if needed
func ensureSchema(ctx context.Context) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating postgres driver: %w", err)
	}

	// Determine migration path based on environment
	migrationPath := getMigrationPath()

	m, err := migrate.NewWithDatabaseInstance(
		migrationPath,
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
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

func getMaxFileSize() int64 {
	maxFileSizeStr := "1024"
	if os.Getenv("MAX_FILE_SIZE_MB") != "" {
		maxFileSizeStr = os.Getenv("MAX_FILE_SIZE_MB")
	}
	maxFileSize, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
	if err != nil {
		errorLogger.Fatalf("Server misconfiguration: invalid MAX_FILE_SIZE_MB %v", err)
	}
	return maxFileSize
}

func getBuildId() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("Failed to generate random id: %v", err)
	}
	return hex.EncodeToString(b)
}

// S3 stuff
func newS3Client() (*s3.Client, error) {
	accessKey := os.Getenv("ACCESS_KEY")
	secretKey := os.Getenv("SECRET_KEY")
	region := os.Getenv("REGION")
	if accessKey == "" || secretKey == "" || region == "" {
		log.Fatal("environment variables ACCESS_KEY, SECRET_KEY and REGION must be set")
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		s3Endpoint = "https://s3." + region + ".scw.cloud"
	}
	// Custom endpoint resolver pointing at Scaleway S3
	endpointResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           s3Endpoint,
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

func getCategories(ctx context.Context) ([]Category, error) {
	// Use the global category repository for backward compatibility
	categoryRepo := NewPostgresCategoryRepository(db)
	return categoryRepo.GetAll(ctx)
}

func getAbout(w http.ResponseWriter, r *http.Request) {
	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/about.html",
	))

	ctx := r.Context()
	var user *User
	if orcid, ok := sessionManager.Get(ctx, "orcid").(string); ok {
		name, _ := sessionManager.Get(ctx, "name").(string)
		user = &User{
			Name:  name,
			Orcid: orcid,
		}
	}

	data := struct {
		App         App
		User        *User
		CurrentPage string
	}{
		App:         app,
		User:        user,
		CurrentPage: "about",
	}

	pageTmpl.ExecuteTemplate(w, "layout", data)
}

func newEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/new.html",
	))

	categories, err := getCategories(r.Context())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Error fetching rows", http.StatusInternalServerError)
		return
	}

	name, _ := sessionManager.Get(ctx, "name").(string)
	user := &User{
		Name:  name,
		Orcid: orcid,
	}

	data := RootPageData{
		App:        app,
		Categories: categories,
		User:       user,
	}

	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", data)
}

func getProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated
	orcid, okO := sessionManager.Get(ctx, "orcid").(string)
	if !okO {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	name, _ := sessionManager.Get(ctx, "name").(string)
	user := &User{
		Name:  name,
		Orcid: orcid,
	}

	// Get user's records
	recordRepo := NewPostgresRecordRepository(db, NewPostgresCategoryRepository(db), NewPostgresRorRepository(db))
	records, totalCount, err := recordRepo.GetAllByOrcidPaginated(ctx, orcid, 100, 0)
	if err != nil {
		http.Error(w, "Error fetching records", http.StatusInternalServerError)
		return
	}

	// Prettify metadata for each record
	for i := range records {
		records[i].MetadataPretty = prettyJSON(records[i].Metadata)
	}

	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/profile.html",
	))

	data := struct {
		App          App
		User         *User
		Records      []Record
		TotalRecords int
	}{
		App:          app,
		User:         user,
		Records:      records,
		TotalRecords: totalCount,
	}

	w.Header().Set("Content-Type", "text/html")
	pageTmpl.ExecuteTemplate(w, "layout", data)
}

// securityHeaders is a middleware that injects your CSP, HSTS, etc.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://cdn.jsdelivr.net; "+
				"style-src 'self' https://cdn.jsdelivr.net; "+
				"img-src 'self' data:; "+
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

	app = App{
		BuildId:     getBuildId(),
		MaxFileSize: getMaxFileSize(),
		Version:     version,
	}

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

	// Session
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

	// Static & healthcheck
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

	// Initialize repositories and handlers
	categoryRepo := NewPostgresCategoryRepository(db)
	adminRepo := NewPostgresAdminRepository(db)
	rorRepo := NewPostgresRorRepository(db)
	recordRepo := NewPostgresRecordRepository(db, categoryRepo, rorRepo)

	categoryHandler := NewCategoryHandler(categoryRepo, adminRepo)
	recordHandler := NewRecordHandler(recordRepo, categoryRepo, adminRepo)
	rorHandler := NewRorHandler()

	// API
	mux.HandleFunc("POST /api/v1/records", recordHandler.CreateRecord)
	mux.HandleFunc("GET /api/v1/record/", recordHandler.Router)
	mux.HandleFunc("POST /api/v1/record/", recordHandler.Router)
	mux.HandleFunc("PUT /api/v1/record/", recordHandler.Router)
	mux.HandleFunc("PATCH /api/v1/record/", recordHandler.Router)
	mux.HandleFunc("DELETE /api/v1/record/", recordHandler.Router)

	// Category API routes
	mux.HandleFunc("/api/v1/categories", categoryHandler.Router)
	mux.HandleFunc("/api/v1/categories/", categoryHandler.Router)

	// ROR API routes
	mux.HandleFunc("/api/v1/ror/search", rorHandler.Router)
	mux.HandleFunc("/api/v1/ror/organizations", rorHandler.Router)
	mux.HandleFunc("/api/v1/ror/organization/", rorHandler.Router)

	// HTML pages (with CSP middleware)
	mux.Handle("/about", securityHeaders(http.HandlerFunc(getAbout)))
	mux.Handle("/profile", securityHeaders(http.HandlerFunc(getProfile)))
	mux.Handle("/record/", securityHeaders(http.HandlerFunc(recordHandler.GetRecordPage)))
	mux.Handle("/entry", securityHeaders(http.HandlerFunc(newEntry)))

	// root catchall - now uses browse page
	mux.Handle("/", securityHeaders(http.HandlerFunc(recordHandler.GetBrowsePage)))

	// TODO use DEV env var to serve files directly to avoid recompilation
	mux.HandleFunc("GET /index.js", serveAsset)
	mux.HandleFunc("GET /robots.txt", serveAsset)
	//http.HandleFunc("GET /index.css", serveAsset)
	mux.HandleFunc("GET /main.css", serveAsset)
	infoLogger.Printf("service running at: %s", siteUrl)

	// Wrap all handlers so they get a request-scoped session context
	handler := sessionManager.LoadAndSave(mux)

	if err := http.ListenAndServe(addr, handler); err != nil {
		errorLogger.Fatalf("failed to start server: %v", err)
	}
}

func getMigrationPath() string {
	if _, err := os.Stat("/sql"); err == nil {
		return "file:///sql"
	}

	if _, err := os.Stat("src/sql"); err == nil {
		return "file://src/sql"
	}

	return "file:///sql"
}
