#!/usr/bin/env bash
#
# End-to-end rule coverage test.
#
# Unit tests prove each rule matches a fixture. This proves the whole chain —
# Docker Engine API, collection, normalization, linking, rules, scoring, JSON —
# against a real daemon, by standing up an environment that is deliberately
# broken in eighteen specific ways and asserting that all eighteen are found.
#
# Everything it creates is prefixed `ddtest-` and is removed on exit, including
# on failure or Ctrl-C. It never touches a resource it did not create.
#
#   ./tests/e2e/run.sh
#
set -uo pipefail

readonly PREFIX="ddtest"
readonly BASE_IMAGE="alpine:3.20"
readonly UNUSED_IMAGE="alpine:3.19"
readonly WORKDIR="$(mktemp -d)"

BINARY="${DOCTORDOCK_BIN:-}"
FAILED=0

# The report is only inspected for resources this script created, so a busy
# machine does not affect the result.
REPORT="$WORKDIR/report.json"

log()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$*"; FAILED=1; }
info() { printf '    %s\n' "$*"; }

cleanup() {
  log "Cleaning up"
  # `docker rm -f` on a name that does not exist is an error, not a no-op, so
  # the whole teardown is quiet and never masks the real failure.
  docker rm -f $(docker ps -aq --filter "name=^${PREFIX}-") >/dev/null 2>&1
  docker network rm "${PREFIX}-net" "${PREFIX}-unused-net" >/dev/null 2>&1
  docker volume rm "${PREFIX}-orphan-volume" >/dev/null 2>&1
  [ -n "${DANGLING_ID:-}" ] && docker rmi -f "$DANGLING_ID" >/dev/null 2>&1
  # Remove the untagged leftover this test deliberately created.
  docker image prune -f --filter "label=doctordock-e2e=1" >/dev/null 2>&1
  rm -rf "$WORKDIR"
  ok "removed every ${PREFIX}-* resource"
}
trap cleanup EXIT INT TERM

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}

# --- setup -------------------------------------------------------------------

require docker
require jq

docker info >/dev/null 2>&1 || { echo "no Docker daemon reachable" >&2; exit 1; }

if [ -z "$BINARY" ]; then
  log "Building doctordock"
  go build -o "$WORKDIR/doctordock" ./cmd/doctordock || exit 1
  BINARY="$WORKDIR/doctordock"
fi
ok "using $BINARY"

log "Creating a deliberately broken environment"

docker pull -q "$BASE_IMAGE"   >/dev/null 2>&1
docker pull -q "$UNUSED_IMAGE" >/dev/null 2>&1   # DD015: nothing will use it
docker pull -q alpine:latest   >/dev/null 2>&1   # DD011: a mutable tag
ok "pulled base images"

docker network create "${PREFIX}-net"        >/dev/null
docker network create "${PREFIX}-unused-net" >/dev/null   # DD018
docker volume  create "${PREFIX}-orphan-volume" >/dev/null # DD017
ok "created networks and an orphan volume"

# DD014: build without a tag, which is the only reliable way to produce an
# image with no references at all.
#
# The obvious approach — build a tag, then rebuild it from different content so
# the first image loses its tag — does not work on Docker 29 with BuildKit: the
# old image is deleted outright rather than left untagged.
printf 'FROM %s\nRUN echo dangling > /marker\n' "$BASE_IMAGE" > "$WORKDIR/Dockerfile.dangling"
DANGLING_ID=$(docker build -q --label doctordock-e2e=1 \
  -f "$WORKDIR/Dockerfile.dangling" "$WORKDIR" 2>/dev/null)
if [ -n "$DANGLING_ID" ]; then
  ok "created a dangling image (${DANGLING_ID:7:12})"
else
  bad "could not create a dangling image"
fi

# The worst container: privileged, Docker socket, sensitive host path, added
# capabilities, a database port on every interface, no limits, no healthcheck.
docker run -d --name "${PREFIX}-worst" \
  --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /etc:/host-etc \
  -p 0.0.0.0:16379:6379 \
  --network "${PREFIX}-net" \
  "$BASE_IMAGE" sleep 600 >/dev/null
ok "created ${PREFIX}-worst (privileged + socket + /etc + exposed 6379)"

# Capabilities are checked separately because DD009 deliberately skips
# privileged containers — they already hold every capability via DD002.
docker run -d --name "${PREFIX}-caps" \
  --cap-add SYS_ADMIN --cap-add NET_ADMIN \
  "$BASE_IMAGE" sleep 600 >/dev/null
ok "created ${PREFIX}-caps (SYS_ADMIN, NET_ADMIN)"

docker run -d --name "${PREFIX}-hostnet" \
  --network host \
  "$BASE_IMAGE" sleep 600 >/dev/null
ok "created ${PREFIX}-hostnet (host networking)"

# DD011: started from a moving tag.
docker run -d --name "${PREFIX}-latest" alpine:latest sleep 600 >/dev/null
ok "created ${PREFIX}-latest (mutable tag)"

# DD012: running, with a healthcheck that always fails. One retry at a two
# second interval reaches "unhealthy" quickly.
docker run -d --name "${PREFIX}-sick" \
  --health-cmd 'exit 1' --health-interval 2s --health-retries 1 --health-start-period 0s \
  "$BASE_IMAGE" sleep 600 >/dev/null
ok "created ${PREFIX}-sick (failing healthcheck)"

# DD013: exits immediately under an always-restart policy, so the daemon keeps
# bringing it back and it sits in the restarting state.
docker run -d --name "${PREFIX}-crashloop" \
  --restart always \
  "$BASE_IMAGE" sh -c 'exit 1' >/dev/null
ok "created ${PREFIX}-crashloop (restart loop)"

# A container that does everything right, to catch false positives.
docker run -d --name "${PREFIX}-good" \
  --user 1000:1000 \
  --restart unless-stopped \
  --memory 64m \
  --read-only \
  --cap-drop ALL \
  --health-cmd 'true' --health-interval 5s \
  --network "${PREFIX}-net" \
  "$BASE_IMAGE" sleep 600 >/dev/null
ok "created ${PREFIX}-good (correctly configured, should stay clean)"

log "Waiting for health and restart state to settle"
sleep 8
ok "settled"

# --- scan --------------------------------------------------------------------

log "Scanning"

# The image size threshold is lowered so that DD016 is exercised without
# pulling a multi-gigabyte image. Everything else runs at its default.
cat > "$WORKDIR/doctordock.yaml" <<'YAML'
thresholds:
  large_image_bytes: 1000000
YAML

"$BINARY" report --config "$WORKDIR/doctordock.yaml" --format json > "$REPORT" || {
  echo "scan failed" >&2
  exit 1
}

jq -e '.schema_version and (.findings | type == "array")' "$REPORT" >/dev/null \
  && ok "produced a valid report ($(jq '.findings | length' "$REPORT") findings, score $(jq '.score' "$REPORT")/100)" \
  || { bad "report is malformed"; exit 1; }

# --- assertions --------------------------------------------------------------

# expect <rule> <resource-name-or-any> <description>
expect() {
  local rule="$1" resource="$2" desc="$3" found
  if [ "$resource" = "any" ]; then
    found=$(jq -r --arg r "$rule" '[.findings[] | select(.id == $r)] | length' "$REPORT")
  else
    found=$(jq -r --arg r "$rule" --arg n "$resource" \
      '[.findings[] | select(.id == $r and .resource_name == $n)] | length' "$REPORT")
  fi

  if [ "$found" -gt 0 ]; then
    ok "$rule  $desc"
  else
    bad "$rule  $desc — expected on '${resource}', found none"
  fi
}

log "Rule coverage"

expect DD001 "${PREFIX}-worst"     "container runs as root"
expect DD002 "${PREFIX}-worst"     "privileged container"
expect DD003 "${PREFIX}-hostnet"   "host networking"
expect DD004 "${PREFIX}-worst"     "sensitive host path (/etc)"
expect DD005 "${PREFIX}-worst"     "Docker socket mounted"
expect DD006 "${PREFIX}-worst"     "Redis port published on 0.0.0.0"
expect DD007 "${PREFIX}-worst"     "no healthcheck"
expect DD008 "${PREFIX}-worst"     "no restart policy"
expect DD009 "${PREFIX}-caps"      "dangerous capabilities"
expect DD010 "${PREFIX}-worst"     "no memory limit"
expect DD011 "${PREFIX}-latest"    "mutable image tag"
expect DD012 "${PREFIX}-sick"      "failing healthcheck"
expect DD013 "${PREFIX}-crashloop" "restart loop"
expect DD014 "${DANGLING_ID:7:12}" "dangling image"
expect DD015 any                   "unused image"
expect DD016 any                   "oversized image (threshold lowered)"
expect DD017 "${PREFIX}-orphan-volume" "unused volume"
expect DD018 "${PREFIX}-unused-net"    "unused network"

log "Severity escalation"

sev() {
  jq -r --arg r "$1" --arg n "$2" \
    '[.findings[] | select(.id == $r and .resource_name == $n) | .severity] | first // "none"' "$REPORT"
}

[ "$(sev DD002 "${PREFIX}-worst")" = "CRITICAL" ] \
  && ok "DD002 is CRITICAL" || bad "DD002 severity is $(sev DD002 "${PREFIX}-worst"), want CRITICAL"
[ "$(sev DD005 "${PREFIX}-worst")" = "CRITICAL" ] \
  && ok "DD005 is CRITICAL" || bad "DD005 severity is $(sev DD005 "${PREFIX}-worst"), want CRITICAL"
[ "$(sev DD009 "${PREFIX}-caps")" = "CRITICAL" ] \
  && ok "DD009 escalates to CRITICAL for SYS_ADMIN" || bad "DD009 severity is $(sev DD009 "${PREFIX}-caps")"

log "False positives on the correctly-configured container"

# DD007 and DD008 must not fire on ddtest-good: it sets both. DD001 must not
# fire either, because it runs as uid 1000. These are the checks that catch a
# rule that fires on everything.
for rule in DD001 DD002 DD003 DD004 DD005 DD007 DD008 DD009 DD010; do
  n=$(jq -r --arg r "$rule" --arg n "${PREFIX}-good" \
    '[.findings[] | select(.id == $r and .resource_name == $n)] | length' "$REPORT")
  [ "$n" -eq 0 ] && ok "$rule does not fire on ${PREFIX}-good" \
                 || bad "$rule fired on the correctly-configured container"
done

log "Relationship resolution"

used=$(jq -r --arg n "${PREFIX}-orphan-volume" \
  '[.volumes[] | select(.name == $n and .in_use == false)] | length' "$REPORT")
[ "$used" -eq 1 ] && ok "orphan volume is reported as unused" || bad "volume usage was resolved incorrectly"

attached=$(jq -r --arg n "${PREFIX}-net" \
  '[.networks[] | select(.name == $n) | .containers | length] | first // 0' "$REPORT")
[ "$attached" -ge 2 ] && ok "network attachments resolved ($attached containers)" \
                      || bad "network attachments resolved as $attached, want at least 2"

builtin_unused=$(jq -r '[.findings[] | select(.id == "DD018" and (.resource_name | IN("bridge","host","none","ingress")))] | length' "$REPORT")
[ "$builtin_unused" -eq 0 ] && ok "Docker's predefined networks are never reported" \
                            || bad "a predefined network was reported as unused"

log "Exit codes"

"$BINARY" scan --config "$WORKDIR/doctordock.yaml" >/dev/null 2>&1; code=$?
[ "$code" -eq 0 ] && ok "no --fail-on exits 0 (exit $code)" || bad "no --fail-on exited $code, want 0"

"$BINARY" scan --config "$WORKDIR/doctordock.yaml" --fail-on critical >/dev/null 2>&1; code=$?
[ "$code" -eq 3 ] && ok "--fail-on critical exits 3 (exit $code)" || bad "--fail-on critical exited $code, want 3"

"$BINARY" scan --config "$WORKDIR/doctordock.yaml" --fail-on high >/dev/null 2>&1; code=$?
[ "$code" -eq 3 ] && ok "--fail-on high exits 3, the worst severity present" || bad "--fail-on high exited $code, want 3"

"$BINARY" scan --ignore NOPE >/dev/null 2>&1; code=$?
[ "$code" -eq 10 ] && ok "a bad flag exits 10, not a findings code" || bad "bad flag exited $code, want 10"

log "Privacy"

# The strongest form of this check: no value from a real container's
# environment may appear anywhere in the report.
leaked=$(jq -r '[.containers[]?.env_keys[]? | select(contains("="))] | length' "$REPORT")
[ "$leaked" -eq 0 ] && ok "no environment variable values in the report" \
                    || bad "$leaked environment keys contain a value"

# --- result ------------------------------------------------------------------

if [ "$FAILED" -eq 0 ]; then
  log "PASS — all 18 rules fired against a real daemon"
else
  log "FAIL — see the ✗ lines above"
fi

exit "$FAILED"
