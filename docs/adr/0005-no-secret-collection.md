# ADR-0005: Environment variable values are never read

- **Status:** accepted
- **Date:** 2026-08-13

## Context

The plan calls for collecting "environment metadata" per container. Container environment
variables are the single most common place for secrets to live: `DATABASE_URL`,
`AWS_SECRET_ACCESS_KEY`, `STRIPE_SECRET_KEY`, JWT signing keys.

DoctorDock writes reports to stdout, to files via `--output`, and — with `--format json` —
into CI logs and artifacts. CI logs are frequently readable by more people than the
production secrets they would contain.

## Decision

The container collector reads environment variable **key names only**. Values are discarded
at the boundary in `internal/docker/containers.go` and never enter `pkg/model`.

`model.Container` has an `EnvKeys []string` field. There is no `EnvValues` field and no
`Env map[string]string` field. The type system makes leaking a value impossible.

## Rationale

Redaction after the fact is the wrong design: it requires every output path to remember to
redact, and it fails open. Not collecting the value at all fails closed — there is no code
path from a secret to a report because the secret is never in memory in a reportable
structure.

Key names alone are sufficient for every rule we plan to write. A v0.2 rule warning that a
container has a variable named `AWS_SECRET_ACCESS_KEY` set needs the name, not the value.

## Consequences

- A rule that needs to inspect a value — for example, entropy-based secret detection —
  cannot be written without revisiting this ADR. That is the intended friction.
- The README states this explicitly. It is a selling point for a tool people will run
  against production hosts.
- Related: DoctorDock makes no network calls and sends no telemetry, so a collected value
  could not leave the machine anyway. This ADR is defence in depth for the local output.
