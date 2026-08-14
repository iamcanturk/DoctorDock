#!/usr/bin/env bash
#
# Publishes a DoctorDock release from this machine.
#
# GitHub Actions is disabled for this repository, so releases are cut locally.
# GoReleaser creates the GitHub release with the darwin/linux binaries and
# checksums, pushes the Homebrew cask to iamcanturk/homebrew-tap, and builds the
# ghcr.io container image. After it finishes, `brew install` works immediately.
#
#   ./scripts/release.sh v0.1.0
#
# The GitHub token comes from `gh`, which already has the scope to write the
# release and to push to the tap repo (both are under the same account).
#
set -euo pipefail

cd "$(dirname "$0")/.."

TAG="${1:-}"
if [ -z "$TAG" ]; then
  echo "usage: $0 vX.Y.Z" >&2
  exit 1
fi
if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "tag must look like v0.1.0, got: $TAG" >&2
  exit 1
fi

echo "==> Verifying everything is green before releasing"
./scripts/verify.sh

echo "==> Logging in to ghcr.io"
gh auth token | docker login ghcr.io -u iamcanturk --password-stdin

echo "==> Tagging $TAG"
git tag -a "$TAG" -m "$TAG"
git push origin "$TAG"

echo "==> Running GoReleaser"
# One token for both the release and the tap push — gh's token has repo scope.
GITHUB_TOKEN="$(gh auth token)" \
HOMEBREW_TAP_TOKEN="$(gh auth token)" \
  goreleaser release --clean

echo
echo "==> Released $TAG"
echo "    brew install iamcanturk/tap/doctordock"
