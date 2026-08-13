# Distribution

Every channel is published from one GoReleaser run triggered by a `v*` tag. The
configuration is [`.goreleaser.yaml`](../.goreleaser.yaml); the workflow is
[`.github/workflows/release.yml`](../.github/workflows/release.yml).

## Channels

| Channel | Platforms | Command |
|---|---|---|
| GitHub Releases | darwin/linux/windows × amd64/arm64 | download the archive |
| Homebrew tap | macOS | `brew install iamcanturk/tap/doctordock` |
| Scoop bucket | Windows | `scoop install doctordock` (no `ddock` alias — Scoop manifests are generated from the archive) |
| ghcr.io | linux/amd64, linux/arm64 | `docker run ghcr.io/iamcanturk/doctordock` |
| npm | any platform Node runs on | `npx doctordock` |
| `go install` | anywhere Go builds | `go install github.com/iamcanturk/DoctorDock/cmd/doctordock@latest` |

Homebrew **casks** are macOS-only. Linux users have `go install`, the container
image, `npx` and the release archives; Linuxbrew support would require a formula
alongside the cask and is not worth the duplication for v0.1.

## One-time setup

Publishing to the tap, the bucket and npm needs three things that do not exist
yet.

### 1. Homebrew tap

Create a public repository named **`homebrew-tap`** under `iamcanturk`. The
`homebrew-` prefix is what makes `brew install iamcanturk/tap/doctordock`
resolve. GoReleaser writes the cask into it; nothing needs to be in it first.

### 2. Scoop bucket

Create a public repository named **`scoop-bucket`**. GoReleaser writes the
manifest into it.

### 3. Secrets

In the DoctorDock repository settings → Secrets and variables → Actions:

| Secret | What it is | Needed for |
|---|---|---|
| `HOMEBREW_TAP_TOKEN` | A PAT with `repo` scope | Pushing to `homebrew-tap` |
| `SCOOP_BUCKET_TOKEN` | A PAT with `repo` scope | Pushing to `scoop-bucket` |
| `NPM_TOKEN` | An npm automation token | Publishing to npm |

`GITHUB_TOKEN` is provided automatically and covers the release itself and
ghcr.io. The other three are separate because the default token cannot write to
*other* repositories.

If `NPM_TOKEN` is absent the npm job skips itself and the rest of the release
still succeeds — npm can be set up later without blocking v0.1.

## Releasing

```bash
make check
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

The workflow then runs the tests, builds six binaries, publishes the release
with checksums and an SBOM, pushes the multi-arch container image, and updates
the tap, the bucket and npm.

Verify the whole pipeline locally without publishing anything:

```bash
make snapshot        # goreleaser release --snapshot --clean
```

## The npm wrapper

`packaging/npm` is a wrapper, not a copy of the tool. On install it downloads
the archive for the current platform from the matching GitHub release, verifies
it against `checksums.txt`, and extracts the binary. `bin/doctordock.js` then
forwards arguments and — importantly — the exit code, so `npx doctordock scan
--fail-on high` still gates a pipeline.

Vendoring six binaries would mean shipping roughly 100 MB to every user so that
they can use one.

The package version is set from the git tag at publish time, which is what makes
the download URL resolve. Its `version` in the repository is a placeholder.

## The container image

Two Dockerfiles exist for a reason:

- **`Dockerfile`** builds from source. It is what `docker build .` uses from a
  clone.
- **`Dockerfile.release`** packages a binary GoReleaser has already built, so
  the release does not compile the same code twice per architecture.

Both run as a non-root user. The tool would otherwise flag its own image under
DD001.

Reading the Docker API needs the socket, and read-only is enough. Because the
image runs as a non-root user, it also needs the socket's group:

```bash
docker run --rm --group-add "$(stat -c '%g' /var/run/docker.sock)" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/iamcanturk/doctordock
```

On Docker Desktop the socket is owned by `root:root` inside the VM, so the
group is `0`:

```bash
docker run --rm --group-add 0 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/iamcanturk/doctordock
```

DoctorDock detects the containerized case and prints the correct command when
the socket is missing or unreadable, so a user who gets it wrong is told
exactly what to run.

## macOS Gatekeeper

The binaries are not notarized, so macOS quarantines them. The Homebrew cask
clears the quarantine attribute on install. Users who download an archive
manually need to do it themselves:

```bash
xattr -dr com.apple.quarantine ./doctordock
```

Notarization needs a paid Apple Developer account and is worth doing before the
macOS app ships in v0.8.

## Not yet published

| Channel | Status |
|---|---|
| GitHub Action | considered for v0.5, alongside SARIF output |
| Nix, AUR, apt, rpm | community-maintained if anyone wants them |
| Linuxbrew formula | only if Linux users ask |
