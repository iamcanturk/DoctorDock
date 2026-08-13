# Contributing to DoctorDock

Thanks for considering a contribution. This document covers what you need to know to get a
change merged.

## Ground rules

DoctorDock has a few constraints that are not negotiable, because they are the reason the
tool exists:

1. **No AI in the analysis path.** Detection, classification and scoring are deterministic
   Go code. AI may only ever consume the JSON output.
2. **No network calls.** The binary must work with the machine offline. No telemetry, no
   update checks, no analytics — ever.
3. **No shelling out to `docker`.** Use the Docker Engine API through `internal/docker`.
4. **No environment variable values.** See [ADR-0005](docs/adr/0005-no-secret-collection.md).
   Key names only.
5. **Read-only.** v0.1 does not mutate the Docker environment. A future `cleanup` command
   will require explicit confirmation.

A PR that breaks one of these will be declined regardless of how good the rest is. If you
think one of them is wrong, open an issue first — that is an ADR-level discussion.

## Setup

```bash
git clone https://github.com/iamcanturk/DoctorDock.git
cd DoctorDock
make build
./bin/doctordock
```

Requires Go 1.24+. A running Docker daemon is needed to *use* the tool, but not to build or
test it.

## Before you open a PR

```bash
make check
```

That runs `go vet`, `gofmt -l`, `go build ./...` and `go test ./...`. CI runs the same
thing on Linux, macOS and Windows.

## Adding a rule

This is the most common contribution and it is designed to be easy.

1. Create the rule in the relevant file under `internal/rules/` — `container_rules.go`,
   `image_rules.go`, `resource_rules.go` or `security_rules.go`.

   ```go
   type MyRule struct{}

   func (MyRule) ID() string              { return "DD019" }
   func (MyRule) Name() string            { return "Short human-readable name" }
   func (MyRule) Category() model.Category { return model.CategorySecurity }
   func (MyRule) Severity() model.Severity { return model.SeverityHigh }

   func (r MyRule) Check(ctx context.Context, t Target) []model.Finding {
       var out []model.Finding
       for _, c := range t.Environment.Containers {
           if !badThing(c) {
               continue
           }
           out = append(out, model.Finding{
               ID:             r.ID(),
               Severity:       r.Severity(),
               Category:       r.Category(),
               Resource:       model.ResourceContainer,
               ResourceID:     c.ID,
               ResourceName:   c.Name,
               Title:          "What is wrong",
               Description:    "Why it matters, in one or two sentences.",
               Recommendation: "What the user should actually do about it.",
           })
       }
       return out
   }
   ```

2. Register it in `internal/rules/registry.go`. One line.
3. Add a test in `internal/rules/*_test.go` using the fixture helpers.
4. Document it in `docs/RULES.md`.

Rule IDs are sequential (`DD019`, `DD020`, …) and are never reused, even if a rule is
removed.

### What makes a good rule

- **Actionable.** The recommendation must tell the user what to do, not just what is wrong.
- **Low false-positive rate.** A rule that fires on every healthy setup trains people to
  ignore output. If a pattern is legitimate in common cases, either narrow the rule or give
  it a lower severity.
- **Justified severity.** `CRITICAL` means "an attacker who reaches this container owns the
  host". Do not inflate.
- **No daemon required to test it.** Rules operate on `model.Environment`; tests build one
  by hand.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/):

```
feat(rules): add DD019 for missing seccomp profile
fix(docker): handle daemons that omit RestartCount
docs: clarify --fail-on semantics
```

## Code style

Standard Go. `gofmt` is enforced. Beyond that:

- Comments explain *why*, not *what*. If the code needs a comment to say what it does,
  rename something instead.
- Errors get context: `fmt.Errorf("list containers: %w", err)`.
- No `panic` outside `main`.
- Exported types in `pkg/model` need doc comments — they are public API.

## Reporting security issues

Do not open a public issue. Email the maintainer directly.
