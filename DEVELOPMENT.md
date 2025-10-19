# Development Setup

This guide covers setting up the eln.community development environment with the CLI tools.

## Quick Start

1. **Start the development environment:**
   ```bash
   make local
   ```

2. **Initialize the database:**
   ```bash
   make cli-migrate
   ```

3. **Seed with sample data:**
   ```bash
   make cli-seed
   ```

4. **Access the application:**
   Open http://localhost:8080 in your browser

## Available Make Commands

### Environment Management
- `make local` - Build and start development environment
- `make build` - Build the Docker image
- `make up` - Start services
- `make down` - Stop services
- `make logs` - Show service logs
- `make clean` - Clean up containers and volumes

### CLI Commands (require running container)
- `make cli-help` - Show CLI help
- `make cli-migrate` - Run database migrations
- `make cli-seed` - Seed database with sample data
- `make cli-list-categories` - List all categories
- `make cli-list-admins` - List all admin users

## Database Migrations

The application uses golang-migrate for database schema management:

- Migration files are in the `src/sql/` directory
- Use `make cli-migrate` to run pending migrations
- Use `docker exec -it eln-community-dev cli db migrate version` to check current version

## CLI Usage

```bash
# Category management
docker exec -it eln-community-dev cli categories add "Chemistry"
docker exec -it eln-community-dev cli categories list

# Admin management
docker exec -it eln-community-dev cli admin add "0000-0002-1825-0097"
docker exec -it eln-community-dev cli admin list

# Database operations
docker exec -it eln-community-dev cli db migrate up
docker exec -it eln-community-dev cli db seed
```