# JSON output contract

`doctordock scan --format json` and `doctordock report` produce the document
described here. It is **public API**: the macOS app, CI pipelines, editor
extensions and any future AI explanation layer decode this shape and nothing
else. See [ADR-0004](adr/0004-json-as-the-gui-contract.md).

## Versioning

Every document carries `schema_version`, versioned independently of the binary.

| Change | Version bump |
|---|---|
| Adding a field | minor (`1.0` → `1.1`) |
| Adding an enum value (a new severity, category or resource kind) | minor |
| Renaming or removing a field | **major** (`1.0` → `2.0`) |
| Changing what an existing field means | **major** |

Clients should refuse a document whose **major** version they do not
understand, and must tolerate unknown fields — that is what makes a minor bump
safe.

Current: **`1.0`**

## Guarantees

- `findings` is always an array, never `null`. Clients can iterate without a
  nil check.
- Every field in `summary` is always present, including zeros.
- `summary.findings.by_category` contains every category, including those with
  a count of zero.
- Timestamps are RFC 3339 in UTC.
- Sizes are bytes. Durations, where present, are seconds.
- Ordering is deterministic: findings are sorted most severe first, then by
  rule ID, then by resource name. Two scans of an unchanged environment produce
  the same order, which is what makes reports diffable.
- Environment variable **values never appear**. `containers[].env_keys` holds
  key names only. See [ADR-0005](adr/0005-no-secret-collection.md).

## Top level

```jsonc
{
  "schema_version": "1.0",
  "generated_at": "2026-08-13T14:19:32.466149Z",
  "tool":    { "name": "doctordock", "version": "0.1.0", "commit": "a1b2c3d" },
  "docker":  { /* daemon information — see below */ },
  "score":   37,
  "summary": { /* aggregate counts — see below */ },
  "findings": [ /* always present, possibly empty */ ],

  // Present with `report` and with `scan --format json`; omitted when the
  // caller asked for a summary only.
  "containers": [], "images": [], "volumes": [], "networks": [],

  // Rule IDs disabled for this run, so a reader can tell "clean" apart from
  // "not checked". Omitted when nothing was skipped.
  "skipped_rules": ["DD007"]
}
```

## `docker`

```jsonc
{
  "server_version":   "29.2.1",
  "api_version":      "1.51",
  "os_type":          "linux",         // the container platform
  "architecture":     "aarch64",
  "kernel_version":   "6.12.68-linuxkit",
  "operating_system": "Docker Desktop",
  "storage_driver":   "overlayfs",
  "cgroup_version":   "2",
  "cpus":             8,
  "mem_total":        8217600000,
  "rootless":         false,
  "security_options": ["seccomp", "cgroupns"]
}
```

## `summary`

```jsonc
{
  "containers": {
    "total": 26, "running": 8, "stopped": 17,
    "paused": 0, "restarting": 1, "created": 0,
    // Counts running containers only: a stopped container keeps whatever
    // health status it had when it stopped.
    "unhealthy": 0
  },
  "images": {
    "total": 29, "dangling": 2, "unused": 7,
    // Images share layers, so total_size over-counts real disk usage.
    "total_size": 13468031315,
    // An upper bound on what a prune would free.
    "reclaimable_size": 4144988878
  },
  "volumes":  { "total": 29, "unused": 18, "anonymous": 13 },
  // "custom" excludes Docker's predefined networks; "unused" only ever
  // counts custom ones.
  "networks": { "total": 12, "custom": 9, "unused": 3 },
  "findings": {
    "total": 95,
    "by_severity": { "info": 50, "low": 22, "medium": 6, "high": 17, "critical": 0 },
    "by_category": { "SECURITY": 22, "PERFORMANCE": 1, "RESOURCE": 11,
                     "CONFIGURATION": 33, "CLEANUP": 28 }
  }
}
```

## `findings[]`

```jsonc
{
  "id":            "DD005",                  // stable rule ID, never reused
  "rule":          "Docker socket exposed",  // identical across findings from one rule
  "severity":      "CRITICAL",               // INFO | LOW | MEDIUM | HIGH | CRITICAL
  "category":      "SECURITY",               // SECURITY | PERFORMANCE | RESOURCE
                                             // | CONFIGURATION | CLEANUP
  "resource":      "container",              // container | image | volume | network | system
  "resource_id":   "01c5b643193...",         // full Docker ID, or volume name
  "resource_name": "api",                    // what to show a human
  "title":         "Docker socket is mounted into the container",
  "description":   "Access to the Docker socket is equivalent to root on the host...",
  "recommendation":"Remove the socket mount...",

  // Rule-specific structured data. Keys vary by rule; clients must tolerate
  // ones they do not recognise. Omitted when a rule provides none.
  "details": { "host_path": "/var/run/docker.sock", "read_only": "false" }
}
```

`severity` and `category` are strings rather than integers precisely so that
adding a level between two existing ones can never renumber the others.

Group by `id` to collapse repeats, and use `rule` as the group heading —
`title` varies per resource for some rules (`"Unused image nginx:1.25 (1.2 GB)"`).

## `containers[]`

```jsonc
{
  "id": "01c5b643...", "name": "cms-mysql",
  "image": "mysql:8.0", "image_id": "sha256:a3dff78d...",
  "state": "running",            // running|exited|created|restarting|paused|removing|dead
  "status": "Up 3 days",
  "created": "2026-03-23T13:56:51Z",
  "started_at": "2026-08-10T06:37:16.255850125Z",

  "ports": [{ "private_port": 3306, "public_port": 3307,
              "type": "tcp", "host_ip": "0.0.0.0" }],
  "mounts": [{ "type": "bind", "source": "/host/path",
               "destination": "/in/container", "read_only": true }],
  "networks": ["app_net"],

  "restart_policy": "unless-stopped", "restart_count": 0,
  "has_healthcheck": false, "health": "none",   // none|starting|healthy|unhealthy

  "user": "",                    // as configured on the container
  "effective_user": "root",      // resolved against the image's USER directive

  "privileged": false,
  "network_mode": "app_net", "pid_mode": "", "ipc_mode": "private",
  "cap_add": [], "cap_drop": [],
  "read_only_rootfs": false,

  "memory_limit": 0,   // bytes; 0 means unlimited
  "nano_cpus": 0,      // units of 1e-9 CPU; 0 means unlimited
  "pids_limit": 0,     // 0 or negative means unlimited

  // Key names only. Values are discarded at collection time.
  "env_keys": ["MYSQL_ROOT_PASSWORD", "PATH"],

  "labels": { "com.docker.compose.project": "cms-docker" }
}
```

A dual-stack publish (`0.0.0.0` and `::` for one `-p`) is collapsed to a single
entry with the IPv4 spelling, so a port is never double-counted.

## `images[]`

```jsonc
{
  "id": "sha256:a3dff78d...",
  "repo_tags": ["mysql:8.0"],          // empty for a dangling image
  "repo_digests": ["mysql@sha256:..."],
  "size": 601234567,                    // bytes
  "shared_size": 8839168,               // bytes shared with other images, or -1
  "created": "2026-03-23T13:56:51Z",
  "architecture": "arm64", "os": "linux", "layers": 7,
  "dangling": false,
  "in_use": true,                       // referenced by any container, running or not
  "used_by": ["cms-mysql"],
  "labels": {}
}
```

`in_use` is resolved from the container list rather than taken from the daemon,
which does not compute it by default. Both the image ID a container was started
from and the reference it names are matched, so a rebuilt tag leaves neither the
old nor the new image looking unused.

## `volumes[]`

```jsonc
{
  "name": "cms-docker_mysql-data", "driver": "local",
  "mountpoint": "/var/lib/docker/volumes/cms-docker_mysql-data/_data",
  "scope": "local", "created": "2026-03-23T13:56:51Z",
  "size": -1,                    // -1 when the daemon did not compute it
  "in_use": true, "used_by": ["cms-mysql"],
  "labels": {}
}
```

## `networks[]`

```jsonc
{
  "id": "3178c32b5560...", "name": "app_net", "driver": "bridge", "scope": "local",
  "created": "2026-08-10T06:37:16.142945042Z",
  "internal": false, "attachable": false, "ipv6": false,
  "containers": ["api", "db"],   // resolved from container network settings
  "subnets": ["172.20.0.0/16"],
  "labels": {}
}
```

## Consuming it

### Shell

```bash
doctordock scan --format json | jq '.score'
doctordock scan --format json | jq '[.findings[] | select(.severity == "CRITICAL")]'
doctordock images --format json | jq '[.images[] | select(.in_use == false)] | map(.size) | add'
```

### Swift

The macOS app mirrors these types as `Codable` structs. Use
`.convertFromSnakeCase` and an ISO-8601 date strategy:

```swift
let decoder = JSONDecoder()
decoder.keyDecodingStrategy = .convertFromSnakeCase
decoder.dateDecodingStrategy = .iso8601

let report = try decoder.decode(Report.self, from: data)
guard report.schemaVersion.hasPrefix("1.") else { throw UnsupportedSchema() }
```

Note that `.iso8601` does not accept fractional seconds; use a custom
`DateFormatter` or `.iso8601withFractionalSeconds` for `generated_at`.

### Go

Decode straight into the source types:

```go
import "github.com/iamcanturk/DoctorDock/pkg/model"

var report model.Report
if err := json.Unmarshal(data, &report); err != nil { /* ... */ }
```

## Compatibility testing

`pkg/model/model_test.go` asserts the presence of every required key by name. A
rename fails the test rather than silently breaking a downstream client.
