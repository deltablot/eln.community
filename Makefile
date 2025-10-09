.PHONY: local build up down logs clean help

# Default target
help:
	@echo "Available targets:"
	@echo "  local     - Build and start local development environment with live reload"
	@echo "  build     - Build the production Docker image"
	@echo "  up        - Start development services with live reload"
	@echo "  down      - Stop all services"
	@echo "  logs      - Show logs from all services"
	@echo "  clean     - Clean up containers, volumes, and images"

# Build and start local development environment with live reload
local: build up
	@echo "🔥 Local development environment with live reload is starting..."
	@echo "📊 MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "🌐 Application: http://localhost:8080"
	@echo "🔄 Live reload enabled - changes will trigger automatic rebuilds"
	@echo "📝 Run 'make logs' to see service logs"

# Build the production Docker image
build:
	@echo "🔨 Building production Docker image..."
	docker build -t ghcr.io/deltablot/eln-community .

# Start services (now uses development by default)
up:
	@echo "🚀 Starting development services with live reload..."
	docker compose -f docker-compose-dev.yml up -d

# Stop services
down:
	@echo "🛑 Stopping services..."
	docker compose -f docker-compose-dev.yml down

# Show logs
logs:
	docker compose -f docker-compose-dev.yml logs -f

# Clean up everything
clean:
	@echo "🧹 Cleaning up development environment..."
	docker compose -f docker-compose-dev.yml down -v
	docker rmi ghcr.io/deltablot/eln-community:dev 2>/dev/null || true
	docker rmi ghcr.io/deltablot/eln-community 2>/dev/null || true
	@echo "✅ Cleanup complete"
