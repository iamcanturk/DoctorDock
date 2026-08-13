# Roadmap

Versions ship when they are useful, not on a calendar. Everything below the line is a
direction, not a promise.

| Version | Theme | Contents |
|---|---|---|
| **v0.1** | Docker diagnostics | Discovery of containers/images/volumes/networks, 18 rules, health score, terminal + JSON output, exit codes |
| v0.2 | Security depth | Seccomp/AppArmor profile checks, user namespace remapping, read-only rootfs, secrets-in-env heuristics (key-name based), rule severity tuning via config |
| v0.3 | Dockerfile analysis | Parse a `Dockerfile` without building: missing `USER`, missing `HEALTHCHECK`, `ADD` vs `COPY`, unpinned base images, layer-count and cache-busting hints |
| v0.4 | Compose analysis | Parse `docker-compose.yml`: privileged services, host mounts, missing restart policies, ports bound to `0.0.0.0`, service dependency sanity |
| v0.5 | CI/CD integration | SARIF output for GitHub code scanning, JUnit XML, a `--diff` mode comparing two reports, prebuilt container image usage docs |
| v0.6 | Performance analysis | Live resource sampling via the stats API, disk usage attribution per image layer, restart-loop and OOM-kill history |
| v0.7 | Interactive TUI | Bubble Tea front-end over the existing report model — a third renderer, not a rewrite |
| v0.8 | macOS app | Native SwiftUI menubar app: live health score in the menubar, resource lists, finding detail views. Consumes the v1 JSON contract |
| v0.9 | VS Code extension | Findings in the Problems panel, Dockerfile/Compose diagnostics inline |
| v1.0 | Stable API | `pkg/model` and the JSON schema frozen under semver |
| v1.x | Optional AI layer | A strictly optional, opt-in explainer that consumes the JSON. The analysis itself stays AI-free forever |

## The AI boundary

```
Docker → DoctorDock Core → Findings → JSON → (optional) AI explanation
```

AI is downstream of everything. It never participates in detection, scoring, or
classification. A DoctorDock scan must produce byte-identical output whether or not any AI
component exists on the machine. This is a permanent architectural constraint, not a v0.1
simplification.

## Distribution channels

| Channel | Status |
|---|---|
| GitHub Releases (darwin/linux/windows × amd64/arm64) | v0.1 |
| Homebrew tap | v0.1 |
| Scoop bucket (Windows) | v0.1 |
| `ghcr.io/iamcanturk/doctordock` | v0.1 |
| `npx doctordock` | v0.1 |
| `go install` | v0.1 (free) |
| GitHub Action | considered for v0.5 |
| Nix / AUR / apt / rpm | community-driven |
