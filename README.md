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

### Get an ORCID ID

To launch the project correctly, you need an ORCID ID.

If you don't have one, go to [orcid.org](https://orcid.org) and create an account.

For ORCID authentication, register your application in the [ORCID Developer Tools](https://orcid.org/developer-tools) and configure the redirect URI to:

```text
{SITE_URL}/auth/callback
```

> **Note**: To launch the app in local mode, you should add the local address with an alias because ORCID does not allow localhost addresses.
> Create a local alias in your `/etc/hosts` file, for example:
>
> ```text
> 127.0.0.1 {LOCALHOST_ALIAS}
> ```
>
> Then configure your ORCID redirect URI as:
>
> ```text
> https://{LOCALHOST_ALIAS}:<port>/auth/callback
> ```

### Launch the project
```bash
# 1. Clone the repository
git clone https://github.com/deltablot/eln-community.git
cd eln-community

# 2. Create a docker-compose.yml from the example
cp docker-compose.yml.dist docker-compose.yml
# Make sure to configure:
# - SITE_URL
# - ORCID credentials
# - S3 settings
# - DEV=1 (when running in dev mode)
# Your ORCID credentials are available at: https://orcid.org/developer-tools

# 3. Start the services
docker compose up -d

# 4. Initialize the database
./cli db migrate up
./cli db seed
./cli admin add <your_orcid>

# 5. Install Air (dev mode only)
# Air is used to automatically rebuild and restart the Go server when files change.
go install github.com/air-verse/air@v1.65.1
# Check that Air is installed correctly
air -v
# If Air not found, add Go's binary directory to your shell PATH
export PATH=$PATH:$(go env GOPATH)/bin
# Then check Air again
air -v

# 6. Run the app in dev mode
air
```

The app should be available at:
```text
https://<LOCALHOST_ALIAS>:<port>
```

## 📚 Documentation

Comprehensive guides are available in the `/docs` folder:

- **[📋 Installation Guide](docs/installation.md)** - Complete setup instructions for all environments
- **[⚙️ Configuration Guide](docs/configuration.md)** - Environment variables and configuration options

