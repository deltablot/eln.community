# Partage

Share files securely.

Try it: https://partage.deltablot.com

## Concepts

* client-side encryption with passphrase
* server knows nothing about the uploaded file
* extremely lightweight and fast
* made with go + html + vanilla js

The idea behind **Partage** is to have something that gets straight to the point, with no extra features, which means some choices have to be made.

No extra fonts, no runtime libraries, no i18n, simple and straightforward interface and codebase.

It is a very simple service to deploy, with a single container of about 10 Mb. There are no database, nothing fancy to setup. You launch the container and it runs.

We believe in lean software, meaning that we only add what is necessary. In the case of **Partage**, there are no runtime javascript dependencies, and a single go dependency (for UUID). The whole source code can be audited easily, because it's so small.

The css and javascript assets are minified and served with brotli compression. The whole application is only a few kb. That's a *small fraction* of the size of files loaded by other similar solutions.

If you're interested in having a file sharing service that is fast, easy to deploy, secure and pleasant to use, then use **Partage**.

### Lighthouse report

![lighthouse report](./assets/lighthouse-report.gif)

### Mozilla's observatory grading

![mozilla observatory](./assets/observatory.png)

## Deploy Partage

### Configuration variables

The application is configured through environment variables.

The only required variable is `SITE_URL`, which corresponds to the public URL of your service.

Configuration variables:

| Name                  | Description                                         | Default             | Example                       |
|-----------------------|-----------------------------------------------------|---------------------|-------------------------------|
| `SITE_URL` (required) | the full URL of the site, as it appears to visitors | http://localhost    | https://partage.deltablot.com |
| `MAX_FILE_SIZE_MB`    | maximum size of uploaded files in Mb                | 1024                | 2048                          |
| `MAX_TOTAL_FILES`     | maximum number of uploaded files                    | 24                  | 1337                          |
| `CLEANUP_TIMER_MIN`   | interval of time in minutes between pruning of old  | 10                  | 60                            |
| `SVG_LOGO`            | svg content for the logo                            | ""                  | `<some svg data>`             |

### Running the service

By default, the program listens on port `8080`. You **need** to have a reverse proxy in front of it, terminating TLS, as it only listens in HTTP, but the app requires access through HTTPS.

~~~bash
# quick and dirty on localhost (no persistence)
docker run -p 8080:8080 -e SITE_URL=http://localhost:8080 --rm --name partage ghcr.io/delatblot/partage
# or with a volume for persistence
mkdir files
# this id/gid corresponds to nobody user in most cases
# it is the userid running inside the container
sudo chown 65534:65534 files
# expose this service through a TLS terminating reverse proxy
docker run -e SITE_URL=https://partage.example.com --rm --name partage -v $(pwd)/files:/var/partage ghcr.io/delatblot/partage
~~~

See [docker-compose.yml](./docker-compose.yml.dist) example file.

## Building image

You can use the `VERSION` build argument to customize the version string.

~~~
docker build --build-arg VERSION=custom -t ghcr.io/deltablot/partage .
~~~

# Technical details

TL;DR: encryption happens locally, the server doesn't know anything about the files you share.

## High level view

### Client-side encryption

Before being sent to the server, the file is locally encrypted with a key derived from the provided passphrase. It is then sent to the server, with some encrypted metadata such as the original file name. This encryption is done using the [SubtleCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto). This means no external library is used, we only rely on browser implementation. **Partage** is only available in secure contexts, which is localhost and https.

### Client-side decryption

To download a file, the correct passphrase must be provided, along with the filename which is present in the hash part of the URL − not sent to the server. The encrypted file is requested from the server, and decrypted locally with the provided passphrase.

## Low level details

The encrypted blob is built by concatenating three things:

header + metadata + file

The `header` stores the length of the metadata using two bytes.

The `metadata` is a JSON object composed of:

- `content_type`: the content-type of the file so we can create a Blob with the same value for download
- `created_at`: the file creation date
- `filename`: original file name

The original file is then added after header and metadata. This blob is then encrypted using a 16 bytes salt and 12 bytes IV. We then concatenate the salt + iv + encrypted blob and send this to the server through a POST request.

This way, the server has no knowledge about the original filename. On the server, the file is stored with a UUIDv7 suffixed with a UNIX timestamp of the expiration time. Periodically, the program will remove files that have expired based on that timestamp stored in the filename. The UUIDv7 makes it nice to `ls` and see older files first, there isn't much more to it.

To decrypt, we ask the server for the file through the `/api/v1/part` endpoint. We know the size of `salt` and `iv` so we can extract them from the blob, and decrypt the file using the derived key from passphrase.

Then a link with a `Blob` is created and clicked to make the browser download it.
