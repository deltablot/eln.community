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
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

var (
	infoLogger  = log.New(os.Stdout, "[info] ", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "[error] ", log.LstdFlags|log.Lshortfile)
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Get database connection
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("set DATABASE_URL")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db.Ping: %v", err)
	}

	// Initialize repositories
	categoryRepo := NewPostgresCategoryRepository(db)
	adminRepo := NewPostgresAdminRepository(db)

	command := os.Args[1]
	switch command {
	case "categories":
		handleCategoriesCommand(ctx, categoryRepo, os.Args[2:])
	case "admin":
		handleAdminCommand(ctx, adminRepo, os.Args[2:])
	case "db":
		handleDBCommand(ctx, db, os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("eln.community CLI - Administrative tool")
	fmt.Println()
	fmt.Println("Usage: cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  categories list                    - List all categories")
	fmt.Println("  categories add <name>              - Add a new category")
	fmt.Println("  categories update <id> <name>      - Update category name")
	fmt.Println("  categories delete <id>             - Delete a category")
	fmt.Println()
	fmt.Println("  admin list                         - List all admin ORCIDs")
	fmt.Println("  admin add <orcid>                  - Add admin ORCID")
	fmt.Println("  admin remove <orcid>               - Remove admin ORCID")
	fmt.Println()
	fmt.Println("  db reset                           - Reset database (WARNING: deletes all data)")
	fmt.Println("  db seed                            - Seed database with sample data")
	fmt.Println("  db migrate up                      - Run all pending migrations")
	fmt.Println("  db migrate down                    - Rollback one migration")
	fmt.Println("  db migrate version                 - Show current migration version")
}

func handleCategoriesCommand(ctx context.Context, repo CategoryRepository, args []string) {
	if len(args) == 0 {
		fmt.Println("Missing categories subcommand")
		printUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		listCategories(ctx, repo)
	case "add":
		if len(args) < 2 {
			fmt.Println("Missing category name")
			os.Exit(1)
		}
		addCategory(ctx, repo, args[1])
	case "update":
		if len(args) < 3 {
			fmt.Println("Missing category ID or name")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			fmt.Printf("Invalid category ID: %s\n", args[1])
			os.Exit(1)
		}
		updateCategory(ctx, repo, id, args[2])
	case "delete":
		if len(args) < 2 {
			fmt.Println("Missing category ID")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			fmt.Printf("Invalid category ID: %s\n", args[1])
			os.Exit(1)
		}
		deleteCategory(ctx, repo, id)
	default:
		fmt.Printf("Unknown categories subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func handleAdminCommand(ctx context.Context, repo AdminRepository, args []string) {
	if len(args) == 0 {
		fmt.Println("Missing admin subcommand")
		printUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		listAdmins(ctx, repo)
	case "add":
		if len(args) < 2 {
			fmt.Println("Missing ORCID")
			os.Exit(1)
		}
		addAdmin(ctx, repo, args[1])
	case "remove":
		if len(args) < 2 {
			fmt.Println("Missing ORCID")
			os.Exit(1)
		}
		removeAdmin(ctx, repo, args[1])
	default:
		fmt.Printf("Unknown admin subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func handleDBCommand(ctx context.Context, db *sql.DB, args []string) {
	if len(args) == 0 {
		fmt.Println("Missing db subcommand")
		printUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	switch subcommand {
	case "reset":
		resetDatabase(ctx, db)
	case "seed":
		seedDatabase(ctx, db)
	case "migrate":
		if len(args) < 2 {
			fmt.Println("Missing migrate subcommand (up/down/version)")
			os.Exit(1)
		}
		handleMigrateCommand(db, args[1:])
	default:
		fmt.Printf("Unknown db subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
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
	category, err := repo.Create(ctx, name)
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
	category, err := repo.Update(ctx, id, name)
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

	// Seed categories
	sampleCategories := []string{
		"Chemistry",
		"Biology",
		"Physics",
		"Materials Science",
		"Environmental Science",
	}

	fmt.Println("Seeding categories...")
	for _, name := range sampleCategories {
		category, err := categoryRepo.Create(ctx, name)
		if err != nil {
			if err == ErrCategoryAlreadyExists {
				fmt.Printf("Category '%s' already exists, skipping\n", name)
				continue
			}
			errorLogger.Printf("Failed to create category '%s': %v", name, err)
			continue
		}
		fmt.Printf("Created category: %s (ID: %d)\n", category.Name, category.Id)
	}

	// Seed admin (you should replace this with actual ORCID)
	sampleAdminOrcid := "0000-0000-0000-0000"
	fmt.Printf("Adding sample admin ORCID: %s\n", sampleAdminOrcid)
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

	fmt.Println("Database seeding completed")
}

func createMigrator(db *sql.DB) (*migrate.Migrate, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func handleMigrateCommand(db *sql.DB, args []string) {
	if len(args) == 0 {
		fmt.Println("Missing migrate subcommand")
		os.Exit(1)
	}

	m, err := createMigrator(db)
	if err != nil {
		errorLogger.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	subcommand := args[0]
	switch subcommand {
	case "up":
		err = m.Up()
		if err != nil && err != migrate.ErrNoChange {
			errorLogger.Fatalf("Failed to run migrations: %v", err)
		}
		if err == migrate.ErrNoChange {
			fmt.Println("No migrations to run")
		} else {
			fmt.Println("Migrations completed successfully")
		}
	case "down":
		err = m.Steps(-1)
		if err != nil && err != migrate.ErrNoChange {
			errorLogger.Fatalf("Failed to rollback migration: %v", err)
		}
		if err == migrate.ErrNoChange {
			fmt.Println("No migrations to rollback")
		} else {
			fmt.Println("Migration rollback completed")
		}
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			errorLogger.Fatalf("Failed to get migration version: %v", err)
		}
		fmt.Printf("Current migration version: %d\n", version)
		if dirty {
			fmt.Println("WARNING: Database is in dirty state")
		}
	default:
		fmt.Printf("Unknown migrate subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}
