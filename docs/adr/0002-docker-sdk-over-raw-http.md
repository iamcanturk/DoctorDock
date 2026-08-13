# ADR-0002: Use the official Docker SDK, not hand-rolled HTTP

- **Status:** accepted
- **Date:** 2026-08-13

## Context

The Docker Engine API is a plain REST API. It would be possible to talk to it with
`net/http` over a Unix socket and cut a large dependency, producing a much smaller binary.

## Decision

Use `github.com/docker/docker/client` (the official SDK).

## Rationale

The parts that look easy are not:

- **Transport varies by platform.** Unix socket on macOS/Linux, named pipe (`npipe://`) on
  Windows, TCP with TLS for remote daemons. Named pipe support alone is a meaningful chunk
  of Windows-specific code.
- **`DOCKER_HOST`, `DOCKER_CONTEXT`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`** all have
  real-world semantics that users expect to work — Colima, Rancher Desktop, Podman's Docker
  socket, remote hosts.
- **API version negotiation.** Engine API versions range widely across installed Docker
  versions. The SDK negotiates down automatically; hand-rolled clients break on old daemons.
- **Response types churn.** Getting them wrong produces silently missing fields rather than
  errors.

Re-implementing all of that correctly is weeks of work and a permanent maintenance tax, for
the benefit of a smaller binary. That is a bad trade for a developer tool.

## Consequences

- Binary lands around 15–20 MB.
- SDK types are confined to `internal/docker`. They are converted to `pkg/model` types at
  the boundary and never escape, so an SDK upgrade touches exactly one package.
- Builds use `-trimpath -ldflags="-s -w"` to trim what can be trimmed.
