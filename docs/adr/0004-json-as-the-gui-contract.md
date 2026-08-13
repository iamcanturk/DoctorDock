# ADR-0004: JSON output is the integration contract for all non-Go clients

- **Status:** accepted
- **Date:** 2026-08-13

## Context

The planned macOS application is a **native SwiftUI menubar app**. Swift cannot import a Go
package. The same is true of the planned VS Code extension (TypeScript) and of any CI
integration.

Alternatives for the macOS app were a Go-based GUI (Wails), which could import the core
directly, or cgo/c-archive bindings. The SwiftUI path was chosen for macOS-native feel and
menubar integration quality.

## Decision

`doctordock scan --format json` produces the single integration contract. Every non-Go
client — macOS app, VS Code extension, CI, dashboards, optional AI layer — consumes that
document and nothing else.

The document carries a `schema_version` field, versioned independently of the binary.

## Rationale

- One contract for all clients means one thing to keep stable, one thing to document, and
  one thing to test.
- Process invocation is a hard isolation boundary: a crash in the core cannot take down the
  menubar app, and the app never needs to be rebuilt when the core is upgraded.
- Users can install a newer `doctordock` via Homebrew and the app keeps working.

## Consequences

- Field names in `pkg/model` are public API. Renaming one is a breaking change requiring a
  `schema_version` bump. This is a real constraint accepted deliberately.
- Everything the terminal renderer displays must also be present in the JSON output. A
  number that exists only in the terminal path is a bug — the macOS app would be unable to
  show it.
- The macOS app ships the `doctordock` binary inside its bundle so it works without a
  separate install, and prefers a newer one on `PATH` if present.
- `schema_version` starts at `1.0` and follows semver: additive field → minor bump,
  removal or rename → major bump.
