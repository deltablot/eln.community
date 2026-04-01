# eln.community

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://www.docker.com/)

A community platform for sharing Electronic Lab Notebook (ELN) archives, experiments, protocols, templates, and research resources. Built to foster collaboration and knowledge sharing in the scientific community.

### ✨ Key Features

- **ELN Archive Sharing**: Upload and share .eln files with the community
- **ORCID Integration**: Secure authentication using ORCID credentials
- **Research Organization Registry (ROR)**: Link uploads to research institutions via [ROR identifiers](https://ror.org/)
- **Categorization**: Organize content with flexible category system
- **RO-Crate Metadata**: Rich metadata support for research objects

### 🛠 Technology Stack

- **Backend**: Go 1.24+ with HTTP server and session management
- **Database**: PostgreSQL for data persistence
- **Storage**: S3-compatible object storage for file uploads
- **Authentication**: ORCID OpenID Connect (OIDC) integration
- **Frontend**: Vanilla JavaScript with esbuild bundling
- **Deployment**: Docker and Docker Compose ready
- **Reverse Proxy**: Nginx configuration included

**Prerequisites**: [Docker](https://docs.docker.com/get-docker/) 20.10+ and [Docker Compose](https://docs.docker.com/compose/install/) 2.0+

## 🚀 Quick Start

### Option 1: Using Makefile (Recommended)
To launch the project correctly, you need an ORCID ID. If you don't have one, go to [orcid.org](https://orcid.org) and create an account.

```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Create a docker-compose-dev.yml with your configuration
cp docker-compose-dev.yml docker-compose.yml
# Set SITE_URL, ORCID credentials, and S3 settings
# Your ORCID credentials are available at https://orcid.org/developer-tools

# 3. Build and start everything
make local
```

### Option 2: Manual Steps

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

### Available Make Commands

- `make local` - Build and start local development environment with live reload
- `make build` - Build the Docker image
- `make up` - Start development services with live reload
- `make down` - Stop all services
- `make logs` - View logs from all services
- `make clean` - Clean up containers, volumes, and images
- `make cli:seed` - Create a fake database 

The default `make local` command now includes live reload using [Air](https://github.com/air-verse/air) - any changes to Go files will automatically trigger a rebuild and restart.

> **Note**: For ORCID authentication, register your application at [ORCID Developer Tools](https://orcid.org/developer-tools) and configure the redirect URI to `{SITE_URL}/auth/callback`.<br>
To launch the app in local mode, you should add the local address with an [alias](https://askubuntu.com/questions/191440/configure-hosts-file-to-use-aliases) because ORCID does not allow localhost addresses, then configure the redirect URI to `{LOCALHOST_ALIAS}:<port>/auth/callback`

## 📚 Documentation

Comprehensive guides are available in the `/docs` folder:

- **[⚙️ Configuration Guide](docs/configuration.md)** - Environment variables and configuration options
- **[📋 Installation Guide](docs/installation.md)** - Complete setup instructions for all environments

