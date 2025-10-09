# eln.community

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://www.docker.com/)

A community platform for sharing Electronic Lab Notebook (ELN) archives, experiments, protocols, templates, and research resources. Built to foster collaboration and knowledge sharing in the scientific community.

## 🔬 About eln.community

eln.community is an open-source platform that enables researchers and scientists to:

- **Share ELN Archives**: Upload and share .eln files containing experiments, protocols, and research data
- **Browse by Category**: Organize and discover content through categorized browsing
- **Secure Authentication**: Login using ORCID credentials for trusted academic identity verification
- **Community Collaboration**: Access shared research resources from the global scientific community

### 🛠 Technology Stack

- **Backend**: Go 1.24+ with HTTP server and session management
- **Database**: PostgreSQL for data persistence
- **Storage**: S3-compatible object storage for file uploads
- **Authentication**: ORCID OpenID Connect (OIDC) integration
- **Frontend**: Vanilla JavaScript with esbuild bundling
- **Deployment**: Docker and Docker Compose ready
- **Reverse Proxy**: Nginx configuration included

## 🚀 Quick Start

Get eln.community running locally in under 5 minutes:

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Clone and Setup
```bash
git clone https://github.com/deltablot/eln-community.git
cd eln-community
cp docker-compose.yml.dist docker-compose.yml
```

### 2. Configure Environment
Edit `docker-compose.yml` and set the required environment variables:
```yaml
environment:
  - SITE_URL=http://localhost:8080
  - ORCID_CLIENT_ID=your_orcid_client_id
  - ORCID_CLIENT_SECRET=your_orcid_client_secret
  # Add your S3 credentials for file storage
```

### 3. Start the Application
```bash
docker compose up -d
```

### 4. Access the Platform
Open your browser and navigate to `http://localhost:8080`

> **Note**: For ORCID authentication to work, you'll need to register your application at [ORCID Developer Tools](https://orcid.org/developer-tools) and configure the client credentials.

## Build

~~~
docker build -t ghcr.io/deltablot/eln-community .
~~~

## Deploy

~~~
# after copying the docker-compose.yml.dist to docker-compose.yml
docker compose up -d
~~~

### Configuration variables

The application is configured through environment variables.

Configuration variables:

| Name                  | Description                                         | Default             | Example                                                 |
|-----------------------|-----------------------------------------------------|---------------------|---------------------------------------------------------|
| `SITE_URL` (required) | the full URL of the site, as it appears to visitors | http://localhost    | https://partage.deltablot.com                           |
| `MAX_FILE_SIZE_MB`    | maximum size of uploaded files in Mb                | 1024                | 2048                                                    |
| `ACCESS_KEY`          | access key of S3 bucket                             | None                | 46ViX7UgQtqd88g                                         |
| `SECRET_KEY`          | secret key of S3 bucket                             | None                | 3a97c858-4e...4735e                                     |
| `BUCKET_NAME`         | s3 bucket name                                      | None                | eln-bucket                                              |
| `REGION`              | region of S3 bucket                                 | None                | fr-par                                                  |
| `DATABASE_URL`        | database connection url                             | None                | postgres://eln:eln@localhost:5432/eln?sslmode=disable   |
| `DEV`                 | enabled dev mode                                    | None                | 1                                                       |
| `ORCID_CLIENT_ID`     | orcid.org OIDC client id                            | None                | 3a97c858-4e...4735e                                     |
| `ORCID_CLIENT_SECRET` | orcid.org OIDC client secret                        | None                | 3a97c858-4e...4735e                                     |

### Running the service

By default, the program listens on port `8080`. You **need** to have a reverse proxy in front of it, terminating TLS, as it only listens in HTTP, but the app requires access through HTTPS.

See [docker-compose.yml](./docker-compose.yml.dist) example file.

## Dev

Debug build:

`docker build --build-arg GO_IMG_TAG=debug -t ghcr.io/deltablot/eln-community .`

### Insert categories

`insert into categories (name) values ('Example')
