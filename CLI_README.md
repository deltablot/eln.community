# eln.community CLI

Administrative command-line tool for managing the eln.community application.

## Usage

The CLI is built into the Docker container and can be accessed via:

```bash
docker exec -it eln.community cli <command> [options]
```

## Commands

### Categories Management
- `cli categories list` - List all categories
- `cli categories add <name>` - Add a new category
- `cli categories update <id> <name>` - Update category name
- `cli categories delete <id>` - Delete a category

### Admin Management
- `cli admin list` - List all admin ORCIDs
- `cli admin add <orcid>` - Add admin ORCID (format: 0000-0000-0000-0000)
- `cli admin remove <orcid>` - Remove admin ORCID

### Database Management
- `cli db migrate up` - Run all pending migrations
- `cli db migrate down` - Rollback one migration
- `cli db migrate version` - Show current migration version
- `cli db reset` - Reset database (WARNING: deletes all data)
- `cli db seed` - Seed database with sample data

## Examples

```bash
# Add a new category
docker exec -it eln.community cli categories add "Organic Chemistry"

# List all categories
docker exec -it eln.community cli categories list

# Add an admin user
docker exec -it eln.community cli admin add "0000-0002-1825-0097"

# Run database migrations
docker exec -it eln.community cli db migrate up

# Seed the database with sample data
docker exec -it eln.community cli db seed
```

## Database Migrations

The application uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema management. Migration files are located in the `migrations/` directory:

- `001_initial_schema.up.sql` - Initial database schema
- `001_initial_schema.down.sql` - Rollback for initial schema

To create new migrations, add files following the naming convention:
`{version}_{description}.{up|down}.sql`