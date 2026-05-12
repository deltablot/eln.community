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

## Configuration Examples

### Development Configuration

```bash
export SITE_URL=http://localhost:8080
export DEV_MODE=1
export DATABASE_URL=postgres://eln:eln@localhost:5432/eln?sslmode=disable
export ORCID_CLIENT_ID=APP-DEV123456789
export ORCID_CLIENT_SECRET=dev-secret-key
export ACCESS_KEY=minioadmin
export SECRET_KEY=minioadmin
export BUCKET_NAME=eln-dev
export REGION=us-east-1
export MAX_FILE_SIZE_MB=100
```

## ORCID Setup

For ORCID authentication to work, you must:

1. Register your application at [ORCID Developer Tools](https://orcid.org/developer-tools)
2. Configure the redirect URI to `{SITE_URL}/auth/orcid/callback`
3. Use the provided client ID and secret in your configuration

## S3 Storage Setup

The application supports any S3-compatible storage service:

### AWS S3
- Create a bucket in your preferred region
- Create IAM user with S3 access permissions
- Use the access key and secret key in configuration

### MinIO (for development)
```bash
docker run -d --name eln-minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"
```

## Environment Files

For local development, create a `.env` file (add to `.gitignore`):

```bash
# .env file for local development
SITE_URL=http://localhost:8080
DEV_MODE=1
DATABASE_URL=postgres://eln:eln@localhost:5432/eln?sslmode=disable
ORCID_CLIENT_ID=your_dev_client_id
ORCID_CLIENT_SECRET=your_dev_client_secret
ACCESS_KEY=minioadmin
SECRET_KEY=minioadmin
BUCKET_NAME=eln-dev
REGION=us-east-1
MAX_FILE_SIZE_MB=100
```

Load the environment file before running:
```bash
source .env
go run src/*.go
```

## Categories

Taken from: https://skos.um.es/unesco6/view.php?l=en&fmt=1
