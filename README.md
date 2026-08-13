<div align="center">

# DoctorDock

**A doctor for your Docker.**

Find the security problems, misconfigurations and wasted disk in your Docker
environment — in under a second, entirely offline.

[![Go Reference](https://pkg.go.dev/badge/github.com/iamcanturk/DoctorDock.svg)](https://pkg.go.dev/github.com/iamcanturk/DoctorDock)
[![Go Report Card](https://goreportcard.com/badge/github.com/iamcanturk/DoctorDock)](https://goreportcard.com/report/github.com/iamcanturk/DoctorDock)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**No AI · No network calls · No telemetry · No account · Nothing deleted without `--apply`**

</div>

---

```
  DoctorDock  0.1.0
  Docker 29.2.1  ·  Docker Desktop  ·  linux/aarch64

  HEALTH SCORE   37/100   poor
  ██████████████░░░░░░░░░░░░░░░░░░░░░░░░░░

  CONTAINERS                          IMAGES
  Total           26                  Total           29  13.5 GB
  Running          8                  Unused           7  4.1 GB
  Stopped         17                  Dangling         2
  Unhealthy        0

  VOLUMES                             NETWORKS
  Total           29                  Total           12
  Unused          18                  Custom           9
  Anonymous       13                  Unused           3

  FINDINGS  1 CRITICAL  ·  17 HIGH  ·  6 MEDIUM  ·  22 LOW
  ──────────────────────────────────────────────────────────────────

  CRITICAL  DD005  Docker socket exposed  1 container
    Docker socket is mounted into the container
      Remove the socket mount. If the container needs the Docker API, put
      a filtering proxy such as docker-socket-proxy in front of it.

  HIGH      DD001  Container runs as root  17 containers
      cms-mysql, cms-nginx, cms-phpmyadmin, crm-postgres, crm-redis
      and 12 more
      Add a non-root USER to the image, or start the container with
      `--user 1000:1000` (compose: `user: "1000:1000"`).

  MEDIUM    DD006  Sensitive port exposed  5 containers
      cms-mysql, crm-postgres, crm-redis, infield-postgres, infield-redis
      Bind the publish to loopback (`-p 127.0.0.1:3307:3306`) if only the
      host needs access.
```

## Why DoctorDock?

Docker makes it easy to run things and hard to notice what you are running.
Six months into a project the average developer machine has a container
mounting the Docker socket, three databases published on `0.0.0.0`, a dozen
containers running as root, and 12 GB of images nothing references. None of it
is visible until something breaks or somebody scans you.

There are tools for parts of this. Docker Bench is a 5,000-line shell script.
Trivy and Grype are excellent at CVEs in images and say nothing about how your
containers are actually configured. `docker system df` tells you disk usage
with no opinion about it.

DoctorDock answers one question: **what is wrong with this Docker environment,
right now, and what should I do about it?**

Design constraints, in order:

- **AI-free.** Every finding is deterministic Go code you can read. The same
  environment always produces the same output. AI, if it ever appears, will
  only ever *explain* the JSON — never produce it.
- **Offline.** Zero network calls. No CVE feed to sync, no account, no update
  check. It works on an air-gapped host and in a locked-down CI runner.
- **No telemetry.** Nothing is collected, transmitted or phoned home.
- **Secrets stay put.** Container environment variables are read as **key names
  only** — values never enter memory in a form that could reach a report.
  ([why](docs/adr/0005-no-secret-collection.md))
- **Nothing disappears by accident.** `doctordock cleanup` is a dry run unless
  you pass `--apply`, and no flag except `--volumes` can ever select a volume —
  not even `--all`. Everything else can be recreated; a volume's contents
  cannot. ([why](docs/adr/0006-cleanup-safety-model.md))
- **Fast.** A full scan of 26 containers, 29 images, 29 volumes and 12 networks
  takes about 550 ms.

## Install

```bash
# macOS
brew install iamcanturk/tap/doctordock

# Windows
scoop bucket add iamcanturk https://github.com/iamcanturk/scoop-bucket
scoop install doctordock

# Any platform, no install
npx doctordock

# Go
go install github.com/iamcanturk/DoctorDock/cmd/doctordock@latest
```

Or run it as a container, with the socket mounted **read-only**:

```bash
docker run --rm --group-add "$(stat -c '%g' /var/run/docker.sock)" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/iamcanturk/doctordock
```

The image runs as a non-root user, so it needs the socket's group. On Docker
Desktop the socket is owned by root, so use `--group-add 0`. DoctorDock detects
this case and prints the right command if you get it wrong.

Prebuilt binaries for macOS, Linux and Windows (amd64 and arm64) are attached to
every [release](https://github.com/iamcanturk/DoctorDock/releases). Everything
is one static binary with no runtime dependencies.

Both `doctordock` and the short alias `ddock` are installed.

## Usage

```bash
doctordock                  # the dashboard — same as `doctordock scan`
doctordock scan             # full scan
doctordock scan --all       # one entry per resource instead of grouped
doctordock security         # security rules only
doctordock containers       # container table + the findings about them
doctordock images           # image table, sizes, what references what
doctordock volumes          # volume table, including anonymous ones
doctordock networks         # network table and attachments
doctordock cleanup          # what could be reclaimed — removes nothing
doctordock cleanup --apply  # actually remove it
doctordock report -o r.json # complete report artifact
doctordock rules            # the rule catalogue
doctordock version --check-docker
```

### Flags

| Flag | What it does |
|---|---|
| `--format json` | Machine-readable output ([schema](docs/JSON_SCHEMA.md)) |
| `--fail-on <level>` | Exit non-zero at this severity or worse |
| `--all` | Do not group repeated findings (on `cleanup`: every target except volumes) |
| `--ignore DD007,DD015` | Skip rules |
| `--only DD001,DD005` | Run only these rules |
| `--no-color` | Disable ANSI colour (also honours `NO_COLOR`) |
| `--config path.yaml` | Use a specific config file |
| `--timeout 30s` | Cap the time spent talking to Docker |

## Security checks

18 rules across five categories. The full catalogue with descriptions is in
[docs/RULES.md](docs/RULES.md); here are the ones that matter most:

| ID | Severity | Check |
|---|---|---|
| `DD005` | `CRITICAL` | Docker socket mounted into a container |
| `DD002` | `CRITICAL` | Container running in privileged mode |
| `DD001` | `HIGH` | Container runs as root |
| `DD004` | `HIGH` | Sensitive host path mounted (`/`, `/etc`, `~/.ssh`, `~/.aws`, …) |
| `DD009` | `HIGH` | Dangerous capabilities added (`SYS_ADMIN`, `SYS_MODULE`, …) |
| `DD003` | `MEDIUM` | Host networking |
| `DD006` | `MEDIUM` | Database or admin port published on `0.0.0.0` |
| `DD012` | `MEDIUM` | Container failing its own healthcheck |
| `DD013` | `MEDIUM` | Container stuck in a restart loop |
| `DD007` | `LOW` | No healthcheck |
| `DD010` | `LOW` | No memory limit |
| `DD014` | `LOW` | Dangling image |
| `DD016` | `LOW` | Oversized image |
| `DD008` | `INFO` | No restart policy |
| `DD011` | `INFO` | Mutable image tag (`:latest`) |
| `DD015` | `INFO` | Unused image |
| `DD017` | `INFO` | Unused volume |
| `DD018` | `INFO` | Unused network |

DoctorDock deliberately does **not** scan for CVEs. That is a solved problem —
use [Trivy](https://github.com/aquasecurity/trivy) or
[Grype](https://github.com/anchore/grype), which need a vulnerability database
and therefore a network. DoctorDock covers the configuration layer those tools
do not look at, and stays offline as a result.

## Cleanup

DoctorDock finds reclaimable resources; `cleanup` removes them. Running it on
its own **never deletes anything** — it prints what it would remove and what
that would free.

```bash
doctordock cleanup                          # dry run: dangling images, unused networks
doctordock cleanup --apply                  # remove them
doctordock cleanup --all                    # also stopped containers and unused images
doctordock cleanup --all --keep-since 24h --apply
doctordock cleanup --volumes                # review unused volumes (still a dry run)
```

Every item is labelled with what removing it could cost:

| Risk | Meaning | Covers |
|---|---|---|
| `safe` | Docker's own prune would remove it | dangling images, unused networks |
| `review` | Removable, but you may have wanted it | unused images, stopped containers |
| `data-loss` | May destroy the only copy of real data | unused volumes |

Four rules make this safe to run:

1. **`--apply` is required.** The verb and the effect are separate; typing the
   obvious command and hitting enter cannot lose data.
2. **`--all` never includes volumes.** It covers containers, images and
   networks. Reaching a volume takes `--volumes`, every time. A plan containing
   volumes asks you to type `delete`, not `y`.
3. **Nothing cascades.** Containers are removed without `-v`, so a container
   never takes anonymous volumes with it — that would route around the
   `--volumes` gate entirely.
4. **Nothing is forced.** If something started using a resource between the scan
   and the apply, the daemon refuses and DoctorDock reports the refusal rather
   than overriding it.

`--keep-since 24h` protects anything created inside the window, so an image you
built this morning survives.

Cleanup also accounts for ordering: an image referenced only by a stopped
container that is *also* being removed counts as unused, so you never have to
run the command twice for it to converge.

## CI/CD usage

Exit codes are **opt-in**. Without `--fail-on`, DoctorDock always exits 0 — a
developer at a shell prompt should not get a non-zero status for having unused
images. CI asks for the gate explicitly:

```bash
doctordock scan --fail-on high
```

| Code | Meaning |
|---|---|
| `0` | Nothing met the threshold |
| `1` | Worst finding was below `HIGH` |
| `2` | At least one `HIGH` |
| `3` | At least one `CRITICAL` |
| `10` | DoctorDock itself failed — bad flags, no daemon, unreadable config |

`10` is separate on purpose: a broken pipeline must never be mistaken for an
insecure environment.

### GitHub Actions

```yaml
- name: Docker health check
  run: |
    docker run --rm --group-add "$(stat -c '%g' /var/run/docker.sock)" \
      -v /var/run/docker.sock:/var/run/docker.sock:ro \
      ghcr.io/iamcanturk/doctordock scan --fail-on high
```

### Track the score over time

```bash
doctordock scan --format json | jq '.score'
```

## Configuration

DoctorDock needs no configuration. When a team wants to share suppressions,
drop a `doctordock.yaml` next to the code:

```yaml
# Rules to skip entirely. Unknown IDs are an error, not a silent no-op.
ignore:
  - DD007   # this project's containers are managed by an external supervisor

# Default --fail-on threshold. "none" disables it.
fail_on: high

thresholds:
  large_image_bytes: 2000000000   # DD016 fires at or above this (default 1.5 GB)
  restart_loop: 5                 # DD013 fires at this restart count
```

Search order: `./doctordock.yaml`, `./.doctordock/doctordock.yaml`, then the
user config directory. A project-local file wins, so a repository can pin its
own settings for everyone who works on it.

## Architecture

```
Docker Engine API
        │
        ▼
   DoctorDock Core (Go)        ← all analysis lives here, deterministic
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

The JSON document is the integration contract for every non-Go client. The
planned macOS menubar app is native SwiftUI and cannot import the Go core, so
the schema is versioned public API from day one.

Adding a rule means writing one type and one registry line. Nothing else in the
codebase changes.

Design decisions are recorded as ADRs:

- [ADR-0001](docs/adr/0001-go-and-cobra.md) — Go + Cobra
- [ADR-0002](docs/adr/0002-docker-sdk-over-raw-http.md) — the official Docker SDK over hand-rolled HTTP
- [ADR-0003](docs/adr/0003-whole-environment-rule-target.md) — rules receive the whole environment
- [ADR-0004](docs/adr/0004-json-as-the-gui-contract.md) — JSON as the GUI contract
- [ADR-0005](docs/adr/0005-no-secret-collection.md) — environment variable values are never read
- [ADR-0006](docs/adr/0006-cleanup-safety-model.md) — cleanup is opt-in, staged by risk, and never cascades

More: [ARCHITECTURE.md](docs/ARCHITECTURE.md) · [JSON_SCHEMA.md](docs/JSON_SCHEMA.md) · [SCORING.md](docs/SCORING.md) · [ROADMAP.md](docs/ROADMAP.md)

## Development

```bash
git clone https://github.com/iamcanturk/DoctorDock.git
cd DoctorDock
make build && ./bin/doctordock
```

### Installing your own build

```bash
make install        # doctordock + the ddock alias onto PATH
make completions    # shell completion for your shell
make uninstall      # remove both
```

`make install` picks the first writable directory already on your `PATH`.
Override it with `make install PREFIX=~/.local/bin`.

### Verifying a change

```bash
make check     # fast: gofmt, vet, build, unit tests
make verify    # everything — run this before tagging a release
```

`make verify` is the release gate. GitHub Actions is **disabled** for this
repository, so verification happens locally and nothing runs in the cloud. It
covers formatting, `go vet`, module tidiness, the generated rule catalogue,
builds for all six release targets, unit tests with and without the race
detector, integration tests against a real daemon, the end-to-end rule coverage
suite, and the GoReleaser config.

Individual pieces:

```bash
make test               # unit tests, no Docker daemon needed
make test-race
make cross              # build all six platform targets
make test-integration   # requires a Docker daemon
make test-e2e           # all 18 rules against a real daemon, creates and removes ddtest-*
make docs               # regenerate docs/RULES.md from the registry
make help               # every target
```

Unit tests need no Docker daemon: `internal/docker.Fake` implements the client
interface from fixtures, which is what makes the cross-platform build check
meaningful without a daemon on each platform.

Requires Go 1.25+ — the Docker SDK pulls in a transitive dependency that needs it.

## Contributing

Adding a rule is the most common contribution and is designed to be easy — see
[CONTRIBUTING.md](CONTRIBUTING.md#adding-a-rule).

A good rule is actionable, has a low false-positive rate, and its
recommendation tells the user what to *do*, not just what is wrong.

The constraints at the top of this README are not negotiable: no AI in the
analysis path, no network calls, no telemetry, no environment variable values,
no mutation. Everything else is open for discussion.

## Roadmap

| Version | Theme |
|---|---|
| **v0.1** | Docker diagnostics — *you are here* |
| v0.2 | Security depth: seccomp/AppArmor profiles, userns, read-only rootfs |
| v0.3 | Dockerfile analysis |
| v0.4 | Docker Compose analysis |
| v0.5 | SARIF output, GitHub code scanning, report diffing |
| v0.6 | Performance analysis via the stats API |
| v0.7 | Interactive terminal UI |
| v0.8 | Native macOS menubar app |
| v0.9 | VS Code extension |
| v1.0 | Stable API |

Full detail in [ROADMAP.md](docs/ROADMAP.md).

## License

MIT — see [LICENSE](LICENSE).
