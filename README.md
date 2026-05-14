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


# Configuration Guide

The application is configured entirely through environment variables. This guide provides comprehensive information about all available configuration options.

## Core Application Settings

| Variable | Required | Default | Description | Example |
|----------|----------|---------|-------------|---------|
| `SITE_URL` | **Yes** | `http://localhost` | Full URL where the application will be accessible. Must include protocol and domain. Used for ORCID redirects and internal links. | `https://eln.community` |
| `DEV_MODE` | No | `0` | Enable development mode. Set to `1` for development features like detailed error messages and hot reload support. | `1` |

## Database Configuration

| Variable | Required | Default | Description | Example |
|----------|----------|---------|-------------|---------|
| `DATABASE_URL` | **Yes** | None | PostgreSQL connection string. Must include username, password, host, port, database name, and SSL mode. | `postgres://eln:password@localhost:5432/eln?sslmode=disable` |

## File Storage (S3) Configuration

| Variable | Required | Default | Description | Example |
|----------|----------|---------|-------------|---------|
| `ACCESS_KEY` | **Yes** | None | S3-compatible storage access key. Required for file upload functionality. | `AKIAIOSFODNN7EXAMPLE` |
| `SECRET_KEY` | **Yes** | None | S3-compatible storage secret key. Keep this secure and never commit to version control. | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` |
| `BUCKET_NAME` | **Yes** | None | Name of the S3 bucket where uploaded files will be stored. Bucket must exist and be accessible. | `eln-community-files` |
| `REGION` | **Yes** | None | AWS region or S3-compatible service region where the bucket is located. | `us-east-1` or `fr-par` |
| `MAX_FILE_SIZE_MB` | No | `1024` | Maximum allowed file size for uploads in megabytes. Adjust based on your storage capacity and needs. | `2048` |

## Authentication (ORCID) Configuration

| Variable | Required | Default | Description | Example |
|----------|----------|---------|-------------|---------|
| `ORCID_CLIENT_ID` | **Yes** | None | ORCID OAuth client ID. Obtain from ORCID Developer Tools after registering your application. | `APP-1234567890ABCDEF` |
| `ORCID_CLIENT_SECRET` | **Yes** | None | ORCID OAuth client secret. Keep secure and never expose in client-side code or logs. | `12345678-1234-1234-1234-123456789abc` |

## S3 Storage Setup

The application supports any S3-compatible storage service:

### AWS S3
- Create a bucket in your preferred region
- Create IAM user with S3 access permissions
- Use the access key and secret key in configuration
