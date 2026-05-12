# Installation Guide

This guide covers different ways to install and set up eln.community for various use cases.

## Prerequisites

Before setting up eln.community, ensure you have the following tools installed:

### Required for All Setups
- **[Docker](https://docs.docker.com/get-docker/)** 20.10+ and **[Docker Compose](https://docs.docker.com/compose/install/)** 2.0+
- **Git** for cloning the repository

### Additional Requirements for Development
- **[Go](https://golang.org/dl/)** 1.24.2+ (as specified in go.mod)
- **[Node.js](https://nodejs.org/)** 18+ with **[Yarn](https://yarnpkg.com/)** 4.6.0+ (for frontend development)
- **[PostgreSQL](https://www.postgresql.org/download/)** 13+ (optional, for local database development)

### External Services
- **ORCID Developer Account**: Register at [ORCID Developer Tools](https://orcid.org/developer-tools) for authentication
- **S3-Compatible Storage**: AWS S3 or compatible service (Localstack, Minio, etc.) for file uploads

## Installation Methods

Choose the setup method that best fits your needs:

### Quick Start (Docker Only)

Get eln.community running in under 5 minutes using Docker:

#### Option 1: Using Makefile (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Edit docker-compose-dev.yml with your configuration
# Set SITE_URL, ORCID credentials, and S3 settings

# 3. Build and start everything
make local
```

#### Option 2: Manual Steps

```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Build the Docker image
docker build -t ghcr.io/deltablot/eln-community .

# 3. Edit docker-compose-dev.yml with your configuration
# Set SITE_URL, ORCID credentials, and S3 settings

# 4. Start the application (includes PostgreSQL database and MinIO)
docker compose -f docker-compose-dev.yml up -d

# 5. Wait for services to be healthy, then access at http://localhost:8080
```

#### Available Make Commands

- `make local` - Build and start local development environment with live reload
- `make build` - Build the Docker image
- `make up` - Start development services with live reload
- `make down` - Stop all services
- `make logs` - View logs from all services
- `make clean` - Clean up containers, volumes, and images

### Development Setup (Recommended)

The default setup now includes live reload using [Air](https://github.com/air-verse/air):

```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Edit docker-compose-dev.yml with your configuration
# Set SITE_URL, ORCID credentials, and S3 settings

# 3. Start development environment with live reload
make local
```

This setup includes:
- Automatic Go application rebuilds on file changes
- PostgreSQL database with persistent data
- MinIO for S3-compatible storage
- Frontend asset building
- All services orchestrated with Docker Compose

### Manual Development Setup

For development without Docker or with custom configurations:

```bash
# 1. Clone and enter directory
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Install Go dependencies
go mod download

# 3. Install frontend dependencies
cd src && yarn install && cd ..

# 4. Set up local PostgreSQL database
createdb eln
psql eln < sql/schema.sql

# 5. Configure environment variables
export SITE_URL=http://localhost:8080
export DEV_MODE=1
export DATABASE_URL=postgres://username:password@localhost:5432/eln?sslmode=disable
export ORCID_CLIENT_ID=your_client_id
export ORCID_CLIENT_SECRET=your_client_secret
# Add S3 configuration as needed

# 6. Build frontend assets
cd src && bash build.sh && cd ..

# 7. Run the application
go run src/*.go
```

## Building

### Docker Build

Build the production Docker image:

```bash
# Standard production build
docker build -t ghcr.io/deltablot/eln-community .

# Debug build (includes shell for troubleshooting)
docker build --build-arg GO_IMG_TAG=debug -t ghcr.io/deltablot/eln-community:debug .
```

### Local Development Build

Build frontend assets for development:

```bash
cd src
yarn install
bash build.sh
cd ..
```

Build and run the Go application:

```bash
go build -o eln-community src/*.go
./eln-community
```

## Next Steps

After installation, you may want to:

- [Configure the application](configuration.md) with your specific settings
