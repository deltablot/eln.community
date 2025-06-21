# eln.community

Source code of https://eln.community.

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
| `REGION`              | region of S3 bucket                                 | None                | fr-par                                                  |
| `DATABASE_URL`        | database connection url                             | None                | postgres://eln:eln@localhost:5432/eln?sslmode=disable   |
| `DEV`                 | enabled dev mode                                    | None                | 1                                                       |
| `ORCID_CLIENT_ID`     | orcid.org OIDC client id                            | None                | 3a97c858-4e...4735e                                     |
| `ORCID_CLIENT_SECRET` | orcid.org OIDC client secret                        | None                | 3a97c858-4e...4735e                                     |

### Running the service

By default, the program listens on port `8080`. You **need** to have a reverse proxy in front of it, terminating TLS, as it only listens in HTTP, but the app requires access through HTTPS.

See [docker-compose.yml](./docker-compose.yml.dist) example file.
