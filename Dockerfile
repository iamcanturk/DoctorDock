# Build DoctorDock from source. The published images are built by GoReleaser
# from a prebuilt binary (see Dockerfile.release); this one exists so that
# `docker build .` works from a clone.

FROM golang:1.25-alpine AS build

WORKDIR /src

# Dependencies are copied first so that a source-only change does not
# re-download the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/doctordock ./cmd/doctordock

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 doctordock

COPY --from=build /out/doctordock /usr/local/bin/doctordock
RUN ln -s /usr/local/bin/doctordock /usr/local/bin/ddock

# DoctorDock only reads from the Docker API, so it runs as a non-root user.
# It practises what it checks: DD001 would flag this image otherwise.
USER doctordock

ENTRYPOINT ["/usr/local/bin/doctordock"]
