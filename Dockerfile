# syntax=docker/dockerfile:1

# Dockerfile for eln.community
# https://github.com/deltablot/eln.community

# must appear before first FROM
# dev: use debug arg to have shell
ARG GO_IMG_TAG=nonroot

# STEP 1
# Node image to minify js and css files + brotli compression
FROM node:23-alpine@sha256:a34e14ef1df25b58258956049ab5a71ea7f0d498e41d0b514f4b8de09af09456 AS bundler
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
FROM golang:1.24-alpine@sha256:8bee1901f1e530bfb4a7850aa7a479d17ae3a18beb6e09064ed54cfd245b7191 AS gobuilder
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -X 'main.version=${VERSION}'" -o /eln.community ./src
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /cli ./src/cmd

# use distroless instead of scratch to have ssl certificates and nobody
FROM gcr.io/distroless/static:${GO_IMG_TAG}
COPY --from=gobuilder /eln.community /usr/local/bin/eln.community
COPY --from=gobuilder /cli /usr/local/bin/cli
COPY --from=gobuilder /app/src/sql /sql
USER nobody:nobody
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/eln.community"]
