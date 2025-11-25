/**
 * eln.community CLI
 * © 2025 - Nicolas CARPi, Deltablot
 * License: AGPLv3
 */
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var (
	errorLogger = log.New(os.Stderr, "[error] ", log.LstdFlags|log.Lshortfile)
	db          *sql.DB
	ctx         context.Context
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "eln.community CLI - Administrative tool",
	Long: `A command-line interface for managing the eln.community application.
This tool provides administrative functions for categories, admin users, and database operations.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize database connection for all commands
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL environment variable must be set")
		}

		var err error
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("Failed to open database connection: %v", err)
		}

		ctx = context.Background()
		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if db != nil {
			db.Close()
		}
	},
}

func init() {
	rootCmd.AddCommand(categoriesCmd)
	rootCmd.AddCommand(adminCmd)
	rootCmd.AddCommand(dbCmd)
}

// Categories command
var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Manage categories",
	Long:  "Commands for managing categories in the eln.community application.",
}

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all categories",
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresCategoryRepository(db)
		listCategories(ctx, repo)
	},
}

var categoriesAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new category",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresCategoryRepository(db)
		addCategory(ctx, repo, args[0])
	},
}

var categoriesUpdateCmd = &cobra.Command{
	Use:   "update <id> <name>",
	Short: "Update category name",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresCategoryRepository(db)
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Printf("Invalid category ID: %s\n", args[0])
			os.Exit(1)
		}
		updateCategory(ctx, repo, id, args[1])
	},
}

var categoriesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a category",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresCategoryRepository(db)
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Printf("Invalid category ID: %s\n", args[0])
			os.Exit(1)
		}
		deleteCategory(ctx, repo, id)
	},
}

var categoriesImportCmd = &cobra.Command{
	Use:   "import <ttl-file>",
	Short: "Import categories from Turtle (TTL) file",
	Long: `Import categories from a Turtle format file (e.g., data/categories.ttl).
This will parse the SKOS hierarchy and create categories with proper parent-child relationships.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresCategoryRepository(db)
		importCategories(ctx, repo, args[0])
	},
}

// Admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage admin users",
	Long:  "Commands for managing admin users in the eln.community application.",
}

var adminListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all admin ORCIDs",
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresAdminRepository(db)
		listAdmins(ctx, repo)
	},
}

var adminAddCmd = &cobra.Command{
	Use:   "add <orcid>",
	Short: "Add admin ORCID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresAdminRepository(db)
		addAdmin(ctx, repo, args[0])
	},
}

var adminRemoveCmd = &cobra.Command{
	Use:   "remove <orcid>",
	Short: "Remove admin ORCID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := NewPostgresAdminRepository(db)
		removeAdmin(ctx, repo, args[0])
	},
}

// Database command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long:  "Commands for database management including migrations, reset, and seeding.",
}

var dbResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset database (WARNING: deletes all data)",
	Run: func(cmd *cobra.Command, args []string) {
		resetDatabase(ctx, db)
	},
}

var dbSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed database with sample data",
	Run: func(cmd *cobra.Command, args []string) {
		seedDatabase(ctx, db)
	},
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration operations",
}

var dbMigrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run all pending migrations",
	Run: func(cmd *cobra.Command, args []string) {
		handleMigrateUp(db)
	},
}

var dbMigrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback one migration",
	Run: func(cmd *cobra.Command, args []string) {
		handleMigrateDown(db)
	},
}

var dbMigrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current migration version",
	Run: func(cmd *cobra.Command, args []string) {
		handleMigrateVersion(db)
	},
}

func init() {
	// Categories subcommands
	categoriesCmd.AddCommand(categoriesListCmd)
	categoriesCmd.AddCommand(categoriesAddCmd)
	categoriesCmd.AddCommand(categoriesUpdateCmd)
	categoriesCmd.AddCommand(categoriesDeleteCmd)
	categoriesCmd.AddCommand(categoriesImportCmd)

	// Admin subcommands
	adminCmd.AddCommand(adminListCmd)
	adminCmd.AddCommand(adminAddCmd)
	adminCmd.AddCommand(adminRemoveCmd)

	// Database subcommands
	dbCmd.AddCommand(dbResetCmd)
	dbCmd.AddCommand(dbSeedCmd)
	dbCmd.AddCommand(dbMigrateCmd)

	// Migration subcommands
	dbMigrateCmd.AddCommand(dbMigrateUpCmd)
	dbMigrateCmd.AddCommand(dbMigrateDownCmd)
	dbMigrateCmd.AddCommand(dbMigrateVersionCmd)
}

// Category operations
func listCategories(ctx context.Context, repo CategoryRepository) {
	categories, err := repo.GetAll(ctx)
	if err != nil {
		errorLogger.Fatalf("Failed to get categories: %v", err)
	}

	if len(categories) == 0 {
		fmt.Println("No categories found")
		return
	}

	fmt.Printf("%-5s %-30s %-20s %-20s\n", "ID", "Name", "Created", "Modified")
	fmt.Println(strings.Repeat("-", 80))
	for _, cat := range categories {
		fmt.Printf("%-5d %-30s %-20s %-20s\n",
			cat.Id,
			cat.Name,
			cat.CreatedAt.Format("2006-01-02 15:04:05"),
			cat.ModifiedAt.Format("2006-01-02 15:04:05"))
	}
}

func addCategory(ctx context.Context, repo CategoryRepository, name string) {
	category, err := repo.Create(ctx, name, nil)
	if err != nil {
		if err == ErrCategoryAlreadyExists {
			fmt.Printf("Category '%s' already exists\n", name)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to create category: %v", err)
	}

	fmt.Printf("Category created successfully:\n")
	fmt.Printf("ID: %d\n", category.Id)
	fmt.Printf("Name: %s\n", category.Name)
	fmt.Printf("Created: %s\n", category.CreatedAt.Format("2006-01-02 15:04:05"))
}

func updateCategory(ctx context.Context, repo CategoryRepository, id int64, name string) {
	// Get existing category to preserve parent_id
	existing, err := repo.GetByID(ctx, id)
	if err != nil {
		if err == ErrCategoryNotFound {
			fmt.Printf("Category with ID %d not found\n", id)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to get category: %v", err)
	}

	category, err := repo.Update(ctx, id, name, existing.ParentId)
	if err != nil {
		if err == ErrCategoryNotFound {
			fmt.Printf("Category with ID %d not found\n", id)
			os.Exit(1)
		}
		if err == ErrCategoryAlreadyExists {
			fmt.Printf("Category name '%s' already exists\n", name)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to update category: %v", err)
	}

	fmt.Printf("Category updated successfully:\n")
	fmt.Printf("ID: %d\n", category.Id)
	fmt.Printf("Name: %s\n", category.Name)
	fmt.Printf("Modified: %s\n", category.ModifiedAt.Format("2006-01-02 15:04:05"))
}

func deleteCategory(ctx context.Context, repo CategoryRepository, id int64) {
	err := repo.Delete(ctx, id)
	if err != nil {
		if err == ErrCategoryNotFound {
			fmt.Printf("Category with ID %d not found\n", id)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to delete category: %v", err)
	}

	fmt.Printf("Category with ID %d deleted successfully\n", id)
}

// Admin operations
func listAdmins(ctx context.Context, repo AdminRepository) {
	admins, err := repo.GetAll(ctx)
	if err != nil {
		errorLogger.Fatalf("Failed to get admins: %v", err)
	}

	if len(admins) == 0 {
		fmt.Println("No admin ORCIDs found")
		return
	}

	fmt.Printf("%-25s %-20s\n", "ORCID", "Created")
	fmt.Println(strings.Repeat("-", 50))
	for _, admin := range admins {
		fmt.Printf("%-25s %-20s\n",
			admin.Orcid,
			admin.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func addAdmin(ctx context.Context, repo AdminRepository, orcid string) {
	admin, err := repo.Add(ctx, orcid)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			fmt.Printf("ORCID '%s' is already an admin\n", orcid)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to add admin: %v", err)
	}

	fmt.Printf("Admin added successfully:\n")
	fmt.Printf("ORCID: %s\n", admin.Orcid)
	fmt.Printf("Created: %s\n", admin.CreatedAt.Format("2006-01-02 15:04:05"))
}

func removeAdmin(ctx context.Context, repo AdminRepository, orcid string) {
	err := repo.Remove(ctx, orcid)
	if err != nil {
		if err == ErrAdminNotFound {
			fmt.Printf("Admin with ORCID '%s' not found\n", orcid)
			os.Exit(1)
		}
		errorLogger.Fatalf("Failed to remove admin: %v", err)
	}

	fmt.Printf("Admin with ORCID '%s' removed successfully\n", orcid)
}

// Database operations
func resetDatabase(ctx context.Context, db *sql.DB) {
	fmt.Print("WARNING: This will delete ALL data in the database. Are you sure? (yes/no): ")
	var confirmation string
	fmt.Scanln(&confirmation)

	if strings.ToLower(confirmation) != "yes" {
		fmt.Println("Operation cancelled")
		return
	}

	m, err := createMigrator(db)
	if err != nil {
		errorLogger.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	// Drop everything
	err = m.Drop()
	if err != nil {
		errorLogger.Fatalf("Failed to drop database: %v", err)
	}

	fmt.Println("Database reset completed")
}

func seedDatabase(ctx context.Context, db *sql.DB) {
	categoryRepo := NewPostgresCategoryRepository(db)
	adminRepo := NewPostgresAdminRepository(db)

	// Import categories from URL
	fmt.Println("Downloading categories from URL...")
	categoriesURL := "https://gist.githubusercontent.com/NicolasCARPi/af0a30a26982b0b9464816edc3e30a6e/raw/2923002c17fc8b93c58768bc07bc7a39f726fa06/unesco6.ttl"

	importCategoriesFromURL(ctx, categoryRepo, categoriesURL)

	// Add sample admin ORCID
	sampleAdminOrcid := "0000-0000-0000-0000"
	fmt.Printf("\nAdding sample admin ORCID: %s\n", sampleAdminOrcid)
	_, err := adminRepo.Add(ctx, sampleAdminOrcid)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			fmt.Printf("Admin ORCID '%s' already exists, skipping\n", sampleAdminOrcid)
		} else {
			errorLogger.Printf("Failed to add admin: %v", err)
		}
	} else {
		fmt.Printf("Added admin ORCID: %s\n", sampleAdminOrcid)
	}

	fmt.Println("\nDatabase seeding completed")
}

func createMigrator(db *sql.DB) (*migrate.Migrate, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	// Determine migration path based on environment
	migrationPath := getMigrationPath()

	m, err := migrate.NewWithDatabaseInstance(
		migrationPath,
		"postgres", driver)
	if err != nil {
		return nil, err
	}

	return m, nil
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

func handleMigrateUp(db *sql.DB) {
	m, err := createMigrator(db)
	if err != nil {
		errorLogger.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		errorLogger.Fatalf("Failed to run migrations: %v", err)
	}
	if err == migrate.ErrNoChange {
		fmt.Println("No migrations to run")
	} else {
		fmt.Println("Migrations completed successfully")
	}
}

func handleMigrateDown(db *sql.DB) {
	m, err := createMigrator(db)
	if err != nil {
		errorLogger.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	err = m.Steps(-1)
	if err != nil && err != migrate.ErrNoChange {
		errorLogger.Fatalf("Failed to rollback migration: %v", err)
	}
	if err == migrate.ErrNoChange {
		fmt.Println("No migrations to rollback")
	} else {
		fmt.Println("Migration rollback completed")
	}
}

func handleMigrateVersion(db *sql.DB) {
	m, err := createMigrator(db)
	if err != nil {
		errorLogger.Fatalf("Failed to get migration version: %v", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		errorLogger.Fatalf("Failed to get migration version: %v", err)
	}
	fmt.Printf("Current migration version: %d\n", version)
	if dirty {
		fmt.Println("WARNING: Database is in dirty state")
	}
}

// TurtleCategory represents a category parsed from Turtle format
type TurtleCategory struct {
	URI       string
	PrefLabel string
	Notation  string
	Broader   string
	Narrower  []string
}

// importCategoriesFromURL imports categories from a URL
func importCategoriesFromURL(ctx context.Context, repo CategoryRepository, url string) {
	fmt.Printf("Downloading categories from %s...\n", url)

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		errorLogger.Fatalf("Failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorLogger.Fatalf("Failed to download file: HTTP %d", resp.StatusCode)
	}

	// Read the content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		errorLogger.Fatalf("Failed to read response: %v", err)
	}

	// Parse Turtle format
	categories := parseTurtleCategories(string(content))
	fmt.Printf("Parsed %d categories from URL\n", len(categories))

	// Create a map to track created categories by notation
	createdCategories := make(map[string]int64)

	// First pass: Create all categories without parent relationships
	fmt.Println("Creating categories...")
	for _, cat := range categories {
		if cat.PrefLabel == "" {
			continue
		}

		// Use English label if available
		name := cat.PrefLabel

		// Check if category already exists
		existing, err := repo.GetByName(ctx, name)
		if err == nil && existing != nil {
			fmt.Printf("Category '%s' already exists (ID: %d), skipping\n", name, existing.Id)
			createdCategories[cat.Notation] = existing.Id
			continue
		}

		// Create category without parent first
		created, err := repo.Create(ctx, name, nil)
		if err != nil {
			if err == ErrCategoryAlreadyExists {
				fmt.Printf("Category '%s' already exists, skipping\n", name)
				continue
			}
			errorLogger.Printf("Failed to create category '%s': %v", name, err)
			continue
		}

		createdCategories[cat.Notation] = created.Id
		fmt.Printf("Created category: %s (ID: %d, Notation: %s)\n", name, created.Id, cat.Notation)
	}

	// Second pass: Set parent relationships
	fmt.Println("\nSetting parent relationships...")
	for _, cat := range categories {
		if cat.Broader == "" {
			continue
		}

		childID, ok := createdCategories[cat.Notation]
		if !ok {
			continue
		}

		// Extract notation from broader URI
		broaderNotation := extractNotation(cat.Broader)
		parentID, ok := createdCategories[broaderNotation]
		if !ok {
			continue
		}

		// Update category with parent
		child, err := repo.GetByID(ctx, childID)
		if err != nil {
			errorLogger.Printf("Failed to get category %d: %v", childID, err)
			continue
		}

		_, err = repo.Update(ctx, childID, child.Name, &parentID)
		if err != nil {
			errorLogger.Printf("Failed to set parent for category %d: %v", childID, err)
			continue
		}

		fmt.Printf("Set parent relationship: %s -> %s\n", cat.Notation, broaderNotation)
	}

	fmt.Printf("\nImport completed. Created/updated %d categories\n", len(createdCategories))
}

// importCategories imports categories from a Turtle file
func importCategories(ctx context.Context, repo CategoryRepository, filePath string) {
	fmt.Printf("Importing categories from %s...\n", filePath)

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		errorLogger.Fatalf("Failed to read file: %v", err)
	}

	// Parse Turtle format
	categories := parseTurtleCategories(string(content))
	fmt.Printf("Parsed %d categories from file\n", len(categories))

	// Create a map to track created categories by notation
	createdCategories := make(map[string]int64)

	// First pass: Create all categories without parent relationships
	fmt.Println("Creating categories...")
	for _, cat := range categories {
		if cat.PrefLabel == "" {
			continue
		}

		// Use English label if available
		name := cat.PrefLabel

		// Check if category already exists
		existing, err := repo.GetByName(ctx, name)
		if err == nil && existing != nil {
			fmt.Printf("Category '%s' already exists (ID: %d), skipping\n", name, existing.Id)
			createdCategories[cat.Notation] = existing.Id
			continue
		}

		// Create category without parent first
		created, err := repo.Create(ctx, name, nil)
		if err != nil {
			if err == ErrCategoryAlreadyExists {
				fmt.Printf("Category '%s' already exists, skipping\n", name)
				continue
			}
			errorLogger.Printf("Failed to create category '%s': %v", name, err)
			continue
		}

		createdCategories[cat.Notation] = created.Id
		fmt.Printf("Created category: %s (ID: %d, Notation: %s)\n", name, created.Id, cat.Notation)
	}

	// Second pass: Set parent relationships
	fmt.Println("\nSetting parent relationships...")
	for _, cat := range categories {
		if cat.Broader == "" {
			continue
		}

		childID, ok := createdCategories[cat.Notation]
		if !ok {
			continue
		}

		// Extract notation from broader URI
		broaderNotation := extractNotation(cat.Broader)
		parentID, ok := createdCategories[broaderNotation]
		if !ok {
			continue
		}

		// Update category with parent
		child, err := repo.GetByID(ctx, childID)
		if err != nil {
			errorLogger.Printf("Failed to get category %d: %v", childID, err)
			continue
		}

		_, err = repo.Update(ctx, childID, child.Name, &parentID)
		if err != nil {
			errorLogger.Printf("Failed to set parent for category %d: %v", childID, err)
			continue
		}

		fmt.Printf("Set parent relationship: %s -> %s\n", cat.Notation, broaderNotation)
	}

	fmt.Printf("\nImport completed. Created/updated %d categories\n", len(createdCategories))
}

// parseTurtleCategories parses categories from Turtle format content
func parseTurtleCategories(content string) []TurtleCategory {
	var categories []TurtleCategory
	var currentCategory *TurtleCategory

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@prefix") {
			continue
		}

		// New concept starts
		if strings.Contains(line, "a skos:Concept") {
			if currentCategory != nil {
				categories = append(categories, *currentCategory)
			}
			currentCategory = &TurtleCategory{
				URI:      extractURI(line),
				Narrower: []string{},
			}
			continue
		}

		if currentCategory == nil {
			continue
		}

		// Parse prefLabel (English)
		if strings.Contains(line, "skos:prefLabel") && strings.Contains(line, "@en") {
			currentCategory.PrefLabel = extractLabel(line)
		}

		// Parse notation
		if strings.Contains(line, "skos:notation") {
			currentCategory.Notation = extractNotation(line)
		}

		// Parse broader (parent)
		if strings.Contains(line, "skos:broader") {
			currentCategory.Broader = extractURI(line)
		}

		// Parse narrower (children)
		if strings.Contains(line, "skos:narrower") {
			currentCategory.Narrower = append(currentCategory.Narrower, extractURI(line))
		}

		// End of concept
		if strings.HasSuffix(line, ".") && currentCategory != nil {
			categories = append(categories, *currentCategory)
			currentCategory = nil
		}
	}

	// Add last category if exists
	if currentCategory != nil {
		categories = append(categories, *currentCategory)
	}

	return categories
}

// extractURI extracts URI from a Turtle line
func extractURI(line string) string {
	// Extract unesco6:XXXX format
	parts := strings.Fields(line)
	for _, part := range parts {
		if strings.HasPrefix(part, "unesco6:") {
			return strings.TrimSuffix(strings.TrimSuffix(part, ";"), ".")
		}
	}
	return ""
}

// extractLabel extracts label from a prefLabel line
func extractLabel(line string) string {
	// Extract text between quotes
	start := strings.Index(line, "\"")
	if start == -1 {
		return ""
	}
	end := strings.Index(line[start+1:], "\"")
	if end == -1 {
		return ""
	}
	return line[start+1 : start+1+end]
}

// extractNotation extracts notation value from a line or URI
func extractNotation(input string) string {
	// If it's a URI like unesco6:1102, extract the number part
	if strings.Contains(input, "unesco6:") {
		parts := strings.Split(input, "unesco6:")
		if len(parts) > 1 {
			return strings.TrimSuffix(strings.TrimSuffix(parts[1], ";"), ".")
		}
	}

	// If it's a notation line like: skos:notation "1102" .
	if strings.Contains(input, "\"") {
		start := strings.Index(input, "\"")
		if start == -1 {
			return ""
		}
		end := strings.Index(input[start+1:], "\"")
		if end == -1 {
			return ""
		}
		return input[start+1 : start+1+end]
	}

	return input
}
