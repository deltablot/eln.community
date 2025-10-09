# Project Structure

Understanding the project layout will help you navigate the codebase and know where to make changes.

## Directory Overview

```
eln.community/
├── 📁 src/                          # Main source code directory
│   ├── 📁 sql/                      # Database schema and migrations
│   │   └── structure.sql            # Main database schema
│   ├── 📁 templates/                # HTML templates (Go templates)
│   │   ├── layout.html              # Base layout template
│   │   ├── index.html               # Homepage template
│   │   ├── browse.html              # Browse/search page template
│   │   ├── record.html              # Individual record view template
│   │   └── about.html               # About page template
│   ├── 📁 dist/                     # Built frontend assets (generated)
│   ├── main.go                      # Main application entry point
│   ├── oidc.go                      # ORCID authentication logic
│   ├── utils.go                     # Utility functions and helpers
│   ├── index.js                     # Frontend JavaScript entry point
│   ├── main.css                     # Main stylesheet
│   ├── build.sh                     # Frontend build script
│   ├── favicon.ico                  # Site favicon
│   └── robots.txt                   # Search engine robots file
├── 📁 docs/                         # Documentation files
├── 📁 nginx/                        # Nginx configuration for production
│   ├── nginx.conf                   # Nginx configuration file
│   ├── 📁 certs/                    # SSL certificates directory
│   └── README.md                    # Nginx setup instructions
├── 📁 .github/                      # GitHub Actions and workflows
│   └── 📁 workflows/                # CI/CD pipeline definitions
├── 📁 assets/                       # Documentation assets
├── docker-compose.yml.dist          # Docker Compose template
├── Dockerfile                       # Docker build configuration
├── go.mod                           # Go module dependencies
├── go.sum                           # Go module checksums
├── package.json                     # Node.js dependencies
├── yarn.lock                        # Yarn dependency lock file
├── CONTRIBUTING.md                  # Development contribution guide
├── LICENSE                          # Project license
└── README.md                        # Main project documentation
```

## Key Directories and Files

### `/src/` - Main Source Code

**Purpose**: Contains all application source code, both backend (Go) and frontend (JS/CSS)

- **`main.go`**: Application entry point, HTTP server setup, routing, and main business logic
- **`oidc.go`**: ORCID OpenID Connect authentication implementation
- **`utils.go`**: Shared utility functions, database helpers, and common operations
- **`index.js`**: Frontend JavaScript for interactive features and API calls
- **`main.css`**: Application styles and responsive design
- **`build.sh`**: Frontend build script using esbuild for bundling and optimization

### `/src/sql/` - Database Schema

**Purpose**: Database structure definitions and migration scripts

- **`structure.sql`**: Complete database schema with tables, indexes, and constraints
- Add new migration files here when modifying the database schema

### `/src/templates/` - HTML Templates

**Purpose**: Go HTML templates for server-side rendering

- **`layout.html`**: Base template with common HTML structure, navigation, and footer
- **`index.html`**: Homepage with upload form and recent submissions
- **`browse.html`**: Search and browse interface with filtering options
- **`record.html`**: Individual ELN file detail view and download page
- **`about.html`**: Project information and usage instructions

### `/docs/` - Documentation

**Purpose**: Comprehensive project documentation split into focused guides

- **`installation.md`**: Installation and setup instructions
- **`configuration.md`**: Environment variables and configuration options
- **`project-structure.md`**: This file - project organization guide

### `/nginx/` - Configuration

**Purpose**: deployment configuration for reverse proxy

- **`nginx.conf`**: Nginx configuration with SSL and proxy settings
- **`certs/`**: Directory for SSL certificates (Let's Encrypt or custom)

### Configuration Files

- **`docker-compose.yml.dist`**: Template for Docker Compose deployment
- - **`docker-compose-local.yml`**: Local Docker Compose deployment
- **`Dockerfile`**: Multi-stage Docker build for production images
- **`go.mod`**: Go module definition with backend dependencies
- **`package.json`**: Node.js dependencies for frontend build tools

## Where to Make Changes

### Adding New Features

- **Backend Logic**: Add to `main.go` or create new `.go` files in `/src/`
- **Frontend Features**: Modify `index.js` and `main.css` in `/src/`
- **New Pages**: Add templates to `/src/templates/` and routes in `main.go`
- **Database Changes**: Create new SQL files in `/src/sql/`

### Styling and UI

- **Global Styles**: Edit `/src/main.css`
- **Page Layout**: Modify `/src/templates/layout.html`
- **Page-Specific UI**: Edit individual template files in `/src/templates/`

### Configuration and Deployment

- **Docker Setup**: Modify `Dockerfile` or `docker-compose.yml.dist`
- **Nginx Config**: Edit `/nginx/nginx.conf` for production proxy settings
- **Dependencies**: Update `go.mod` for Go packages or `package.json` for Node.js tools

### Documentation

- **Installation Guide**: Update `/docs/installation.md`
- **Configuration**: Update `/docs/configuration.md`
- **Main README**: Update root `README.md` for overview changes

### Development Tools

- **Build Process**: Modify `/src/build.sh` for frontend build customization
- **CI/CD**: Edit `.github/workflows/` for automated testing and deployment

## Code Organization Principles

### Backend (Go)

- Keep HTTP handlers in `main.go`
- Put authentication logic in `oidc.go`
- Add utility functions to `utils.go`
- Create new files for major feature areas

### Frontend (JavaScript/CSS)

- Use vanilla JavaScript for simplicity
- Keep CSS organized with clear class naming
- Minimize external dependencies
- Use esbuild for bundling and optimization

### Templates (HTML)

- Extend `layout.html` for consistent page structure
- Use Go template syntax for dynamic content
- Keep templates focused on presentation logic
- Separate concerns between templates and business logic

## File Naming Conventions

### Go Files
- Use lowercase with underscores for multi-word files: `user_handler.go`
- Keep related functionality in the same file
- Use descriptive names that indicate purpose

### Frontend Files
- Use kebab-case for CSS classes: `.upload-form`
- Use camelCase for JavaScript variables: `uploadButton`
- Keep asset files in appropriate directories

### Templates
- Use lowercase with hyphens: `user-profile.html`
- Match template names to their primary purpose
- Keep template files focused on single pages or components

### Database Files
- Use descriptive names with version numbers: `001_initial_schema.sql`
- Include migration direction in filename: `002_add_categories_up.sql`
- Keep schema changes in separate files for tracking

## Development Workflow Integration

The project structure supports efficient development workflows:

1. **Hot Reload**: `DEV=1` serves assets directly from `/src/`
2. **Build Process**: `build.sh` creates optimized assets in `/src/dist/`
3. **Database Migrations**: SQL files in `/src/sql/` for schema changes
4. **Documentation**: Focused docs in `/docs/` for different aspects
5. **Configuration**: Environment-based config with examples in docs
