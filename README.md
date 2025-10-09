# eln.community

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://www.docker.com/)

A community platform for sharing Electronic Lab Notebook (ELN) archives, experiments, protocols, templates, and research resources. Built to foster collaboration and knowledge sharing in the scientific community.

### 🛠 Technology Stack

- **Backend**: Go 1.24+ with HTTP server and session management
- **Database**: PostgreSQL for data persistence
- **Storage**: S3-compatible object storage for file uploads
- **Authentication**: ORCID OpenID Connect (OIDC) integration
- **Frontend**: Vanilla JavaScript with esbuild bundling
- **Deployment**: Docker and Docker Compose ready
- **Reverse Proxy**: Nginx configuration included

## 🚀 Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Edit docker-compose-local.yml with your configuration
# Set SITE_URL, ORCID credentials, and S3 settings

# 3. Start the application (includes PostgreSQL database)
docker compose up -f docker-compose-local.yml -d

# 4. Wait for services to be healthy, then access at http://localhost:8080
```

**Prerequisites**: [Docker](https://docs.docker.com/get-docker/) 20.10+ and [Docker Compose](https://docs.docker.com/compose/install/) 2.0+

> **Note**: For ORCID authentication, register your application at [ORCID Developer Tools](https://orcid.org/developer-tools) and configure the redirect URI to `{SITE_URL}/auth/orcid/callback`.

## 📚 Documentation

Comprehensive guides are available in the `/docs` folder:

- **[📋 Installation Guide](docs/installation.md)** - Complete setup instructions for all environments
- **[⚙️ Configuration Guide](docs/configuration.md)** - Environment variables and configuration options
- **[💻 Development Guide](docs/development.md)** - Development workflow and best practices
- **[📁 Project Structure](docs/project-structure.md)** - Codebase organization and navigation

