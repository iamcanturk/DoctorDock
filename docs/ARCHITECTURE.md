# Architecture

## Layers

```
cmd/doctordock          thin main(); delegates to internal/cli
internal/cli            Cobra commands, flag parsing, exit codes
internal/report         Terminal + JSON renderers
internal/scanner        Orchestration: collect → evaluate → aggregate
internal/rules          Rule interface, registry, DD001–DD018
internal/score          Scorer interface + default weighted scorer
internal/docker         Client interface + Docker Engine API implementation
internal/config         Optional config file loading
pkg/model               Public data model — the JSON contract
```

Dependency direction is strictly downward. `pkg/model` imports nothing from the project.
`internal/rules` imports only `pkg/model`. Nothing below `internal/report` knows what a
terminal is.

## Why an interface in front of Docker

```go
type Client interface {
	Ping(ctx context.Context) (model.DockerInfo, error)
	ListContainers(ctx context.Context) ([]model.Container, error)
	ListImages(ctx context.Context) ([]model.Image, error)
	ListVolumes(ctx context.Context) ([]model.Volume, error)
	ListNetworks(ctx context.Context) ([]model.Network, error)
	Close() error
}
```

Two reasons, both concrete:

1. **Unit tests must not need a Docker daemon.** CI runs on three operating systems;
   requiring a live daemon on all of them is fragile. `internal/docker.Fake` implements
   this interface from static fixtures.
2. **Normalization happens at the boundary.** The Docker SDK's types churn between
   versions. Converting to `pkg/model` types in exactly one place means an SDK upgrade
   touches one package, not the whole rule set.

The interface returns `pkg/model` types, not SDK types. That is the whole point — no SDK
type is allowed to escape `internal/docker`.

## Rule engine

```go
type Rule interface {
	ID() string
	Name() string
	Category() model.Category
	Severity() model.Severity
	Check(ctx context.Context, target Target) []model.Finding
}

type Target struct {
	Environment *model.Environment
}
```

`Target` carries the whole environment snapshot rather than a single resource. Rules like
"unused image" or "unused network" inherently need to cross-reference containers, and
splitting the interface per-resource-type would force a second interface for exactly those
rules. One shape, no special cases.

Adding a rule is: write a type implementing `Rule`, add one line to the registry in
`internal/rules/registry.go`. Nothing else changes.

`Severity()` on the interface is the rule's *default* severity. A rule may still emit a
finding at a different severity when the situation warrants it (e.g. DD004 escalates for
`/` versus a merely sensitive path); the interface value is what documentation and
`doctordock rules` display.

## Scoring

```go
type Scorer interface {
	Calculate(findings []model.Finding) int
}
```

The default implementation subtracts per-severity weights from 100 and clamps to [0, 100].
It is deliberately trivial and deliberately isolated, so that a better model (diminishing
returns, category weighting, resource-count normalization) can replace it without touching
the scanner.

## Output

`internal/report` holds two renderers over the same `model.Report`:

- `terminal.go` — ANSI, respects `NO_COLOR` and non-TTY output
- `json.go` — the versioned contract consumed by CI, the macOS app, and any future client

Adding a Bubble Tea TUI later means adding a third renderer, not restructuring anything.

## The macOS app boundary

The SwiftUI app is a separate native target. It cannot link the Go core, so it invokes the
`doctordock` binary with `scan --format json` and decodes the result into `Codable` structs
mirroring `pkg/model`.

This is why `schema_version` exists and why the JSON field names are treated as API. See
[JSON_SCHEMA.md](JSON_SCHEMA.md) and [ADR-0004](adr/0004-json-as-the-gui-contract.md).

## Privacy posture

DoctorDock reads container configuration, which includes environment variables — the single
most likely place for secrets to sit. The collector reads **key names only** and never the
values. There is no code path that can place an environment variable value into a report.
See [ADR-0005](adr/0005-no-secret-collection.md).
