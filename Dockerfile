# syntax=docker/dockerfile:1

# Dockerfile for partage
# https://github.com/deltablot/partage

# STEP 1
# Node image to minify js and css files + brotli compression
FROM node:23-alpine AS bundler
RUN corepack enable \
    && corepack prepare yarn@stable --activate
RUN apk add --no-cache brotli bash
WORKDIR /home/node
USER node
COPY --chown=node:node src src
COPY package.json src/
COPY yarn.lock src/
WORKDIR /home/node/src
RUN yarn install
RUN bash build.sh

# STEP 2
# Go builder
FROM golang:1.24-alpine AS gobuilder
# this is set at build time
ARG VERSION=docker
WORKDIR /app
# install dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download
# copy code
COPY src src
COPY --from=bundler /home/node/src/dist src/dist
# disable CGO or it doesn't work in scratch
# target linux/amd64
# -w turn off DWARF debugging
# -s turn off symbol table
# change version at linking time
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -X 'main.partageVersion=${VERSION}'" -o /partage ./src/main.go

# use busybox to create a correctly chown dir
FROM busybox:1.37-uclibc AS prepare
RUN mkdir -p /var/partage \
    && chown nobody:nobody /var/partage

# use distroless instead of scratch to have ssl certificates and nobody
# dev: use :debug tag to have shell
#FROM gcr.io/distroless/static:debug
FROM gcr.io/distroless/static:nonroot
# copy the pre‑owned directory
COPY --from=prepare --chown=nobody:nobody /var/partage /var/partage
COPY --from=gobuilder /partage /usr/local/bin/partage
USER nobody:nobody
EXPOSE 8080
WORKDIR /srv/partage
ENTRYPOINT ["/usr/local/bin/partage"]
