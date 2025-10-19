# eln.community CLI

Administrative command-line tool for managing the eln.community application, built with [Cobra CLI](https://github.com/spf13/cobra).

## Usage

The CLI is built into the Docker container and can be accessed via:

```bash
docker exec -it eln-community-dev cli <command> [options]
```

Or using the convenient Makefile commands:

```bash
make cli-help          # Show CLI help
make cli-migrate       # Run database migrations
make cli-seed          # Seed database with sample data
```

## Commands

### Categories Management
```bash
cli categories list                    # List all categories
cli categories add <name>              # Add a new category
cli categories update <id> <name>      # Update category name
cli categories delete <id>             # Delete a category
```

### Admin Management
```bash
cli admin list                         # List all admin ORCIDs
cli admin add <orcid>                  # Add admin ORCID (format: 0000-0000-0000-0000)
cli admin remove <orcid>               # Remove admin ORCID
```

### Database Management
```bash
cli db migrate up                      # Run all pending migrations
cli db migrate down                    # Rollback one migration
cli db migrate version                 # Show current migration version
cli db reset                           # Reset database (WARNING: deletes all data)
cli db seed                            # Seed database with sample data
```

## Examples

### Using Docker directly:
```bash
# Add a new category
docker exec -it eln-community-dev cli categories add "Organic Chemistry"

# List all categories
docker exec -it eln-community-dev cli categories list

# Add an admin user
docker exec -it eln-community-dev cli admin add "0000-0002-1825-0097"

# Run database migrations
docker exec -it eln-community-dev cli db migrate up

# Seed the database with sample data
docker exec -it eln-community-dev cli db seed
```

### Using Makefile shortcuts:
```bash
# Initialize database and seed with sample data
make cli-migrate
make cli-seed

# List categories and admins
make cli-list-categories
make cli-list-admins

# Check database migration version
make cli-db-version
```

## Getting Help

The CLI includes comprehensive help for all commands:

```bash
# General help
docker exec -it eln-community-dev cli --help

# Help for specific commands
docker exec -it eln-community-dev cli categories --help
docker exec -it eln-community-dev cli admin --help
docker exec -it eln-community-dev cli db --help
```

## Database Migrations

The application uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema management. Migration files are located in the `migrations/` directory:

- `001_initial_schema.up.sql` - Initial database schema
- `001_initial_schema.down.sql` - Rollback for initial schema

To create new migrations, add files following the naming convention:
`{version}_{description}.{up|down}.sql`

## Development Workflow

1. Start the development environment:
   ```bash
   make local
   ```

2. Initialize the database:
   ```bash
   make cli-migrate
   ```

3. Seed with sample data:
   ```bash
   make cli-seed
   ```

4. Access the application at http://localhost:8080