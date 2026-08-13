# ADR-0001: Go + Cobra for the CLI

- **Status:** accepted
- **Date:** 2026-08-13

## Context

DoctorDock must ship as a single binary on macOS, Linux and Windows, start fast enough to
feel instant, and talk to the Docker Engine API. It must be installable without a runtime.

## Decision

Go, with Cobra for command routing.

## Rationale

- Go cross-compiles to a static binary for all six target platform/arch pairs from a single
  CI job. No runtime, no dependency hell for users.
- The Docker Engine and the Docker CLI are themselves written in Go; the official SDK is
  first-class and the API types are the authoritative ones.
- Cobra is what `docker`, `kubectl`, `gh` and `helm` use. Users get the subcommand and flag
  behaviour they already expect, plus shell completion for free.
- Startup time is single-digit milliseconds, which matters for a tool people will run
  dozens of times a day.

## Alternatives considered

- **Rust + clap.** Comparable binary story and smaller output, but the Docker API client
  ecosystem is thinner and less authoritative, and it would put the future Wails/Go path
  out of reach.
- **Node/Python.** Rejected outright: requires a runtime, slow startup, painful packaging.

## Consequences

- Binary size lands around 15–20 MB because of the Docker SDK. Acceptable for a developer
  tool; noted in [ADR-0002](0002-docker-sdk-over-raw-http.md).
- A future Bubble Tea TUI (v0.7) stays in-language.
