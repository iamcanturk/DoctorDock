#!/usr/bin/env bash
#
# Everything CI would run, locally.
#
# GitHub Actions is disabled for this repository, so this script is the release
# gate. It is deliberately strict: a green run here is what "verified" means.
#
#   make verify
#
# Set SKIP_DOCKER=1 to skip the checks that need a running daemon.
#
set -uo pipefail

FAILED=0
START=$(date +%s)

bold()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()    { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()   { printf '  \033[31m✗\033[0m %s\n' "$*"; FAILED=1; }
skip()  { printf '  \033[2m–\033[0m %s\n' "$*"; }

# step <description> <command...>
step() {
  local desc="$1"; shift
  local out
  if out=$("$@" 2>&1); then
    ok "$desc"
  else
    bad "$desc"
    printf '%s\n' "$out" | sed 's/^/      /' | head -25
  fi
}

cd "$(dirname "$0")/.." || exit 1

bold "Formatting and static analysis"

unformatted=$(gofmt -l . 2>/dev/null)
if [ -z "$unformatted" ]; then
  ok "gofmt"
else
  bad "gofmt — these files need formatting:"
  printf '%s\n' "$unformatted" | sed 's/^/      /'
fi

step "go vet"                go vet ./...
step "go vet (integration)"  go vet -tags integration ./tests/...

bold "Module hygiene"

# `go mod tidy` must be a no-op on a clean tree, or the committed go.mod does
# not describe what the code actually imports.
cp go.mod /tmp/dd-go.mod.bak && cp go.sum /tmp/dd-go.sum.bak
if go mod tidy >/dev/null 2>&1 && diff -q go.mod /tmp/dd-go.mod.bak >/dev/null && diff -q go.sum /tmp/dd-go.sum.bak >/dev/null; then
  ok "go.mod and go.sum are tidy"
else
  bad "go mod tidy changed go.mod or go.sum — commit the result"
  cp /tmp/dd-go.mod.bak go.mod && cp /tmp/dd-go.sum.bak go.sum
fi
rm -f /tmp/dd-go.mod.bak /tmp/dd-go.sum.bak

bold "Generated documentation"

# Compare the generator's output to the file on disk rather than to git HEAD,
# so the check means "the committed catalogue is current" in a dirty tree too.
cp docs/RULES.md /tmp/dd-rules.bak 2>/dev/null
if go run ./tools/gendocs >/dev/null 2>&1 && diff -q /tmp/dd-rules.bak docs/RULES.md >/dev/null 2>&1; then
  ok "docs/RULES.md matches the rule registry"
else
  bad "docs/RULES.md was out of date — it has been regenerated, commit the change"
fi
rm -f /tmp/dd-rules.bak

bold "Build"

# Every release target, because a platform-specific mistake compiles fine here
# and breaks for a user on another OS.
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  if GOOS="${target%/*}" GOARCH="${target#*/}" go build -o /dev/null ./... 2>/dev/null; then
    ok "builds for $target"
  else
    bad "does not build for $target"
    GOOS="${target%/*}" GOARCH="${target#*/}" go build -o /dev/null ./... 2>&1 | sed 's/^/      /' | head -10
  fi
done

bold "Tests"

step "unit tests"                go test ./...
step "unit tests (race)"         go test -race ./...

if [ "${SKIP_DOCKER:-}" = "1" ]; then
  bold "Docker-dependent checks"
  skip "skipped (SKIP_DOCKER=1)"
elif ! docker info >/dev/null 2>&1; then
  bold "Docker-dependent checks"
  skip "skipped — no Docker daemon reachable"
else
  bold "Against a real Docker daemon"
  step "integration tests"  go test -tags integration ./tests/integration/...

  if ./tests/e2e/run.sh >/tmp/dd-e2e.log 2>&1; then
    ok "end-to-end rule coverage (all 18 rules)"
  else
    bad "end-to-end rule coverage"
    grep -E '✗' /tmp/dd-e2e.log | sed 's/^/    /' | head -20
  fi
  rm -f /tmp/dd-e2e.log
fi

bold "Release pipeline"

if command -v goreleaser >/dev/null 2>&1; then
  step "goreleaser config" goreleaser check
elif docker info >/dev/null 2>&1; then
  if docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:latest check >/dev/null 2>&1; then
    ok "goreleaser config (via container)"
  else
    bad "goreleaser config"
    docker run --rm -v "$PWD":/src -w /src goreleaser/goreleaser:latest check 2>&1 | sed 's/^/      /' | tail -10
  fi
else
  skip "goreleaser not available"
fi

ELAPSED=$(( $(date +%s) - START ))

if [ "$FAILED" -eq 0 ]; then
  bold "PASS — verified in ${ELAPSED}s"
else
  bold "FAIL — see the ✗ lines above (${ELAPSED}s)"
fi

exit "$FAILED"
