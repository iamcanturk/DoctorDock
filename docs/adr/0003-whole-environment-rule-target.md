# ADR-0003: Rules receive the whole environment, not a single resource

- **Status:** accepted
- **Date:** 2026-08-13

## Context

The obvious rule interface is per-resource:

```go
Check(ctx, container model.Container) []Finding
```

It reads well for DD001 ("container runs as root"). It falls apart for DD015 ("unused
image"), which must know every container's image reference, and for DD018 ("unused
network"), which must know every container's network attachments.

## Decision

```go
type Target struct {
	Environment *model.Environment
}

Check(ctx context.Context, target Target) []model.Finding
```

Every rule receives the full snapshot and iterates whatever it needs.

## Rationale

The per-resource interface would force a second interface for cross-referencing rules, and
then a third dispatch mechanism to run both. One shape with no special cases is smaller
than two shapes with a bridge between them.

The cost — each rule writes its own `for` loop over `target.Environment.Containers` — is
three lines of obvious code per rule. That is cheaper than the abstraction it replaces.

## Consequences

- Adding a rule never requires touching the engine.
- `Target` is a struct rather than a bare `*model.Environment` precisely so that v0.3 and
  v0.4 can add `Dockerfile` and `Compose` fields without changing a single existing rule
  signature.
- A rule can, in principle, be quadratic over the environment. Environments have tens of
  resources, not millions; the scanner builds shared lookup indexes on the `Environment`
  for the cases that matter (image usage, network attachment, volume usage).
