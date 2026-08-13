# DoctorDock — Execution Plan

> Living document. Each step produces **working, building, tested code** before the next
> one starts. Status is updated as work lands.

## Product definition

DoctorDock is a **local-first, AI-free, zero-telemetry** Docker diagnostics CLI. Running
`doctordock` prints a readable health report of the local Docker environment: resource
counts, a health score, security findings, and optimization suggestions.

Non-goals for v0.1: AI, CVE databases, automatic cleanup, GUI, telemetry, cloud.

## Naming

| Thing | Value |
|---|---|
| Product | DoctorDock |
| Primary binary | `doctordock` |
| Short alias | `ddock` (identical binary, shipped as a second name) |
| Go module | `github.com/iamcanturk/DoctorDock` |
| Container image | `ghcr.io/iamcanturk/doctordock` |
| npm package | `doctordock` |

## Core principles (hard constraints)

1. No AI in the analysis path — ever. AI may only consume the JSON output, never produce it.
2. No network calls. No telemetry. No user data collection. The binary works fully offline.
3. Docker Engine API only. Shell-outs to the `docker` CLI are forbidden.
4. Cross-platform: macOS, Linux, Windows. Single static binary.
5. Environment **variable values are never read** — only key names, and only for metadata.
   See [ADR-0005](adr/0005-no-secret-collection.md).
6. Read-only by default. v0.1 never mutates the Docker environment.
7. No unnecessary abstraction. Interfaces exist where they are needed for testing or for a
   known future consumer — nowhere else.

## Step plan

| # | Step | Status |
|---|---|---|
| 1 | Repository, license, contributing, plan docs | ✅ done |
| 2 | Go module + directory skeleton | ✅ done |
| 3 | `pkg/model` — public data model (the JSON contract) | ✅ done |
| 4 | `internal/docker` — `Client` interface + Engine API implementation | ✅ done |
| 5 | Docker connection smoke test (`doctordock version --check-docker`) | ✅ done |
| 6 | Container discovery (list + inspect + normalize) | ✅ done |
| 7 | Image discovery | ✅ done |
| 8 | Volume discovery | ✅ done |
| 9 | Network discovery | ✅ done |
| 10 | `Finding` model, severity, category | ✅ done |
| 11 | Rule engine + registry | ✅ done |
| 12 | Rules DD001–DD018 | ✅ done |
| 13 | Scanner engine (collect → evaluate → aggregate) | ✅ done |
| 14 | `internal/score` — pluggable scorer | ✅ done |
| 15 | Terminal report renderer | ✅ done |
| 16 | JSON report renderer (versioned schema) | ✅ done |
| 17 | CLI: root, scan, containers, images, volumes, networks, security, report, version | ✅ done |
| 18 | Exit codes + `--fail-on` | ✅ done |
| 19 | Config file + rule ignore | ✅ done |
| 20 | Unit tests with a fake Docker client | ✅ done |
| 21 | GitHub Actions: test / vet / build | ✅ done |
| 22 | GoReleaser: binaries, Homebrew tap, Scoop, ghcr.io image | ✅ done |
| 23 | npm wrapper package | ✅ done |
| 24 | README + docs | ✅ done |

## Architecture boundary that matters most

The macOS app is **SwiftUI native**, so it cannot import the Go core. The integration
boundary is therefore the JSON document produced by `doctordock scan --format json`.

```
Docker Engine API
        │
        ▼
   DoctorDock Core (Go)          ← all analysis lives here
        │
        ▼
   Report (pkg/model)
        │
   ┌────┴─────────────┐
   ▼                  ▼
Terminal renderer   JSON renderer  ← versioned, stable contract
                      │
              ┌───────┼────────┬──────────────┐
              ▼       ▼        ▼              ▼
            CI/CD  macOS app  VS Code    Optional AI layer
                   (SwiftUI)  extension
```

Consequences, enforced from day one:

- `pkg/model` types carry explicit, stable `json` tags. Renaming a field is a breaking change.
- The JSON document carries `schema_version`. See [JSON_SCHEMA.md](JSON_SCHEMA.md).
- No terminal/ANSI concern ever leaks into `internal/scanner`, `internal/rules` or `pkg/model`.
- Every number the terminal renderer prints must also exist in the JSON output.

## Release gate for v0.1.0

- [x] Connects to Docker on macOS / Linux / Windows via the Engine API
- [x] Discovers containers, images, volumes, networks
- [x] 18 rules across security, configuration, resource and cleanup categories
- [x] Health score
- [x] Terminal output
- [x] JSON output with `schema_version`
- [x] Exit codes and `--fail-on`
- [x] Unit tests that do not require a Docker daemon
- [x] CI green on Linux / macOS / Windows
- [x] README

## Roadmap

See [ROADMAP.md](ROADMAP.md).
