.PHONY: local build up down logs clean help cli-help cli-categories cli-admin cli-db

# Default target
help:
	@echo "Available targets:"
	@echo "  local         - Build and start local development environment"
	@echo "  build         - Build the production Docker image"
	@echo "  up            - Start development services"
	@echo "  down          - Stop all services"
	@echo "  logs          - Show logs from all services"
	@echo "  clean         - Clean up containers, volumes, and images"
	@echo ""
	@echo "CLI Commands (requires running container):"
	@echo "  cli-help      - Show CLI help"
	@echo "  cli-migrate   - Run database migrations"
	@echo "  cli-seed      - Seed database with sample data"
	@echo "  cli-categories- Show category management commands"
	@echo "  cli-admin     - Show admin management commands"

# Build and start local development environment
local: build up
	@echo "Local development environment is starting..."
	@echo "MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "Application: http://localhost:8080"
	@echo "Run 'make logs' to see service logs"
	@echo "Run 'make cli-migrate' to initialize the database"

# Build the production Docker image
build:
	@echo "Building production Docker image..."
	docker build -t ghcr.io/deltablot/eln-community .

# Start services
up:
	@echo "Starting development services..."
	docker compose -f docker-compose-dev.yml up -d

# Stop services
down:
	@echo "Stopping services..."
	docker compose -f docker-compose-dev.yml down

# Show logs
logs:
	docker compose -f docker-compose-dev.yml logs -f

# Clean up everything
clean:
	@echo "Cleaning up development environment..."
	docker compose -f docker-compose-dev.yml down -v
	docker rmi ghcr.io/deltablot/eln-community:dev 2>/dev/null || true
	docker rmi ghcr.io/deltablot/eln-community 2>/dev/null || true
	@echo "Cleanup complete"

# CLI Commands - these require the container to be running
cli-help:
	@echo "Running CLI help..."
	docker exec -it eln-community-dev cli --help

cli-migrate:
	@echo "Running database migrations..."
	docker exec -it eln-community-dev cli db migrate up

cli-seed:
	@echo "Seeding database with sample data..."
	docker exec -it eln-community-dev cli db seed

cli-categories:
	@echo "Category management commands:"
	docker exec -it eln-community-dev cli categories --help

cli-admin:
	@echo "Admin management commands:"
	docker exec -it eln-community-dev cli admin --help

cli-db:
	@echo "Database management commands:"
	docker exec -it eln-community-dev cli db --help

# Quick CLI shortcuts
cli-list-categories:
	docker exec -it eln-community-dev cli categories list

cli-list-admins:
	docker exec -it eln-community-dev cli admin list

cli-db-version:
	docker exec -it eln-community-dev cli db migrate version