# Rule catalogue

Every check DoctorDock runs, with its default severity and what it looks for.

**This file is generated from the rule registry** by `make docs`, so it cannot
drift from the code.

Rule IDs are stable and are **never reused**, even if a rule is removed — a
suppression written today keeps meaning the same thing after an upgrade.

## Severity

| Level | Meaning | Score weight |
|---|---|---|
| `CRITICAL` | Reaching this container is equivalent to owning the host | 25 |
| `HIGH` | A real weakness that should be fixed | 15 |
| `MEDIUM` | Weakens the setup, but needs another factor to be exploited | 8 |
| `LOW` | A minor deviation from good practice | 3 |
| `INFO` | Worth knowing, not a problem | 0 |

Weights are the cost of the *first* finding from a given rule; repeats of the
same rule decay. See [SCORING.md](SCORING.md).

A rule's severity below is its **default**. Some rules escalate for specific
situations — DD004 reports a writable host-root mount as `CRITICAL` rather than
`HIGH`, and DD006 does the same for an exposed Docker API port.

## Suppressing a rule

```bash
doctordock scan --ignore DD007,DD015
```

or, for a whole team, in `doctordock.yaml`:

```yaml
ignore:
  - DD007
```

Suppressed rules are listed in the JSON output under `skipped_rules`, so a
reader can always tell "clean" apart from "not checked".

## Understanding a rule

This table is a summary. Any rule explains itself in full:

```bash
doctordock explain DD005
```

That covers what it looks for, why it matters, a worked scenario, fixes you can
copy, when it is fine to ignore, and links to the upstream documentation.

## Security

Weaknesses an attacker could exploit.

| ID | Severity | Rule | What it looks for |
|---|---|---|---|
| `DD001` | `HIGH` | Container runs as root | Reports containers that start as uid 0, either because no USER is set on the container or because the image itself does not set one. |
| `DD002` | `CRITICAL` | Privileged container | Reports containers started with --privileged, which grants all capabilities and removes device and namespace restrictions. |
| `DD003` | `MEDIUM` | Host networking | Reports containers using --network=host, which removes network isolation between the container and the host. |
| `DD004` | `HIGH` | Sensitive host path mounted | Reports bind mounts of host paths that expose system state or credentials, such as /etc, /var/lib/docker or ~/.ssh. |
| `DD005` | `CRITICAL` | Docker socket exposed | Reports containers with the Docker socket mounted, which is equivalent to giving them root on the host. |
| `DD006` | `MEDIUM` | Sensitive port exposed | Reports database, message-broker and admin ports published on all interfaces rather than bound to loopback. |
| `DD009` | `HIGH` | Dangerous capabilities added | Reports containers granted Linux capabilities that weaken or defeat container isolation, such as SYS_ADMIN or SYS_MODULE. |

## Configuration

Setups that are fragile or simply wrong.

| ID | Severity | Rule | What it looks for |
|---|---|---|---|
| `DD007` | `LOW` | No healthcheck | Reports containers with no HEALTHCHECK, so Docker cannot tell a hung process from a healthy one. |
| `DD008` | `INFO` | No restart policy | Reports containers with no restart policy, which stay down after a crash or a host reboot. |
| `DD011` | `INFO` | Mutable image tag | Reports containers running from a moving tag such as :latest, which makes the deployment non-reproducible. |

## Resource

Limits, quotas and consumption.

| ID | Severity | Rule | What it looks for |
|---|---|---|---|
| `DD010` | `LOW` | No memory limit | Reports running containers with no memory limit, which can consume all host memory and trigger the OOM killer against unrelated processes. |
| `DD016` | `LOW` | Oversized image | Reports images above the configured size threshold (1.5 GB by default). |

## Performance

Settings and states that degrade runtime behaviour.

| ID | Severity | Rule | What it looks for |
|---|---|---|---|
| `DD012` | `MEDIUM` | Container is unhealthy | Reports containers whose own healthcheck is currently failing. |
| `DD013` | `MEDIUM` | Container restart loop | Reports containers that have been restarted repeatedly, or that are stuck in the restarting state. |

## Cleanup

Reclaimable, unused resources.

| ID | Severity | Rule | What it looks for |
|---|---|---|---|
| `DD014` | `LOW` | Dangling image | Reports untagged images left behind when a rebuild moved a tag to a new image. |
| `DD015` | `INFO` | Unused image | Reports tagged images that no container references, and the disk they occupy. |
| `DD017` | `INFO` | Unused volume | Reports volumes that no container mounts. These are reported only — DoctorDock never deletes a volume, because an unused volume can still hold the only copy of real data. |
| `DD018` | `INFO` | Unused network | Reports user-defined networks with no attached containers. Docker's predefined networks are never reported. |

---

18 rules in total. Adding one is documented in [CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-rule).
