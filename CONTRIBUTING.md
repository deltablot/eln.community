# Developer documentation

## First step

~~~bash
git clone git@github.com:deltablot/eln.community.git
cd eln.community
~~~

## ORCID setup


### Get an ORCID

To launch the project correctly, you need an ORCID.

If you don't have one, go to [orcid.org](https://orcid.org) and create an account.

### OAuth config

Login to your orcid account, go into developer tools from the top right user menu. Take note of client id/secret for integration in your env later. Use these settings:

- Application name: eln.community dev
- Application url: eln.community.local
- Application description: local eln.community dev
- Redirect URIs: https://eln.community.local:8081/auth/callback

### Local alias

**Note**: To launch the app in local mode, you should add the local address with an alias because ORCID does not allow localhost addresses.
Create a local alias in your `/etc/hosts` file, for example:

```text
127.0.0.1 eln.community.local
```



## nginx https

ORCID Oauth needs to redirect to an https server, so we need to run nginx (in a container), with self-signed certs, as a reverse proxy.

Generate the certs with `mkcert`: https://github.com/filosottile/mkcert

~~~bash
cd nginx
mkdir certs
cd certs
mkcert eln.community.local
mv eln.community.local.pem server.crt
mv eln.community.local-key.pem server.key
~~~

## start nginx + postgres

~~~bash
cp docker-compose.yml.dist docker-compose.yml
# comment out the eln-community service: we will not use docker for dev
# docker services are just the database and nginx
$EDITOR docker-compose.yml
docker compose up -d
~~~

# Start local server

This will run server on port 8080 and save the files in the `files` directory.

## Environment Files

For local development, create a `.env` file:

~~~bash
# .env file example for local development
SITE_URL=https://eln.community.local:8081
DEV_MODE=1
DATABASE_URL=postgres://eln:eln@localhost:5432/eln?sslmode=disable
ORCID_CLIENT_ID=<your_dev_client_id>
ORCID_CLIENT_SECRET=<your_dev_client_secret>
# s3 related env are not absolutely necessary in dev if you run it with --files option to save files locally
ACCESS_KEY=<your_ak>
SECRET_KEY=<your_sk>
BUCKET_NAME=eln-community-dev
REGION=fr-par
MAX_FILE_SIZE_MB=100
~~~

The `DEV_MODE=1` environment variable will make the program serve .js, .html and .css files directly, instead of embedding them in the binary, for faster iteration.

Load the environment file before running:

## Start server

```bash
source .env
go run src/*.go --dir files
# use --port option to run on another port than the default
```

## Initialize the database

~~~bash
./cli db migrate up
./cli db seed
./cli admin add <your_orcid>
~~~

# Categories

Taken from: https://skos.um.es/unesco6/view.php?l=en&fmt=1

# Docker build

### Local Development Build

~~~bash
# Install air to automatically rebuild and restart the Go server when files change.
go install github.com/air-verse/air@v1.65.1
# Check that Air is installed correctly
air -v
# If Air not found, add Go's binary directory to your shell PATH
export PATH=$PATH:$(go env GOPATH)/bin
# Then check Air again
air -v
# Run the app
air
~~~

### Docker Build

Build the production Docker image:

```bash
# Standard production build
docker build -t ghcr.io/deltablot/eln-community .

# Debug build (includes shell for troubleshooting)
docker build --build-arg GO_IMG_TAG=debug -t ghcr.io/deltablot/eln-community:debug .
```
