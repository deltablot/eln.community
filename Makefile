.PHONY: local build up down logs clean help

# Default target
help:
	@echo "Available targets:"
	@echo "  local  - Build and start local development environment"
	@echo "  build  - Build the Docker image"
	@echo "  up     - Start services with docker-compose"
	@echo "  down   - Stop and remove services"
	@echo "  logs   - Show logs from all services"
	@echo "  clean  - Clean up containers, volumes, and images"

# Build and start local development environment
local: build up
	@echo "🚀 Local environment is starting..."
	@echo "📊 MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "🌐 Application: http://localhost:8080"
	@echo "📝 Run 'make logs' to see service logs"

# Build the Docker image
build:
	@echo "🔨 Building Docker image..."
	docker build -t ghcr.io/deltablot/eln-community .

# Start services
up:
	@echo "🚀 Starting services..."
	docker compose -f docker-compose-local.yml up -d

# Stop services
down:
	@echo "🛑 Stopping services..."
	docker compose -f docker-compose-local.yml down

# Show logs
logs:
	docker compose -f docker-compose-local.yml logs -f

# Clean up everything
clean:
	@echo "🧹 Cleaning up..."
	docker compose -f docker-compose-local.yml down -v
	docker rmi ghcr.io/deltablot/eln-community 2>/dev/null || true
	@echo "✅ Cleanup complete"