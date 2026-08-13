# doctordock

Local-first Docker diagnostics. Find security problems, misconfigurations and
reclaimable resources in your Docker environment.

**No AI · No network calls · No telemetry · Read-only**

```bash
npx doctordock
```

This package is a thin wrapper: on install it downloads the DoctorDock binary
for your platform from the matching GitHub release and verifies it against the
release checksums. Arguments and exit codes are forwarded unchanged, so

```bash
npx doctordock scan --fail-on high
```

still gates a CI pipeline.

Full documentation: https://github.com/iamcanturk/DoctorDock

MIT licensed.
