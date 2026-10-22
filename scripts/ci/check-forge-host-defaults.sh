#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-forge-host-defaults.sh — no script may send FORGEJO_TOKEN to a host
# the user did not name.
#
# Two checks:
#   1. Static: no script under scripts/ defaults FORGEJO_URL (or any other
#      http(s) URL) in a file that also sends an Authorization header.
#   2. Behavioural: each forge-talking script exits non-zero, with a message
#      naming FORGEJO_URL, and makes no curl request when FORGEJO_URL is unset
#      (curl is replaced by a stub that logs its calls). With FORGEJO_URL set,
#      every request goes to that host.
#
# POSIX sh, run by `make check` through scripts/ci/*.sh.

set -eu

self="$(basename "$0")"
root="$(cd "$(dirname "$0")/../.." && pwd)"
fail=0

# ── 1. static guard ─────────────────────────────────────────────────────
offenders=""
for f in $(find "$root/scripts" -name '*.sh' ! -name "$self"); do
  # A default has a non-empty value: ${FORGEJO_URL:-} is only an emptiness test.
  if grep -q 'Authorization' "$f" && grep -Eq 'FORGEJO_URL:-[^}]|:-https?://' "$f"; then
    offenders="$offenders ${f#"$root"/}"
  fi
done
if [ -n "$offenders" ]; then
  echo "✗ static: defaulted host next to an Authorization header in:$offenders"
  fail=1
else
  echo "✓ static: no script defaults a host beside an Authorization header"
fi

# ── 2. behavioural: stub curl, run each forge-talking script ───────────
tmp="$(mktemp -d "${TMPDIR:-/tmp}/forge-host-defaults.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
cat > "$tmp/bin/curl" <<'EOF'
#!/bin/sh
echo "$*" >> "$STUB_LOG"
echo '{}'
EOF
chmod +x "$tmp/bin/curl"
log="$tmp/curl.log"

run() {
  # $1 = script, rest = extra env assignments and args for it
  script="$1"
  shift
  : > "$log"
  rc=0
  out="$(env -u FORGEJO_URL FORGEJO_TOKEN=sentinel-token \
    PATH="$tmp/bin:$PATH" STUB_LOG="$log" "$@" sh "$root/$script" 1 2>&1)" || rc=$?
  printf '%s\n' "$out" > "$tmp/out"
}

for script in scripts/ci/check-castra-release-onboarding.sh scripts/ci/check-pr-ci-ran.sh; do
  name="$(basename "$script")"

  # FORGEJO_URL unset: refuse, say why, send nothing.
  run "$script"
  if [ "$rc" -eq 0 ]; then
    echo "✗ $name: exited 0 with FORGEJO_URL unset"
    fail=1
  elif ! grep -q 'FORGEJO_URL' "$tmp/out"; then
    echo "✗ $name: refusal does not name FORGEJO_URL"
    fail=1
  elif [ -s "$log" ]; then
    echo "✗ $name: made a request with FORGEJO_URL unset: $(cat "$log")"
    fail=1
  else
    echo "✓ $name: refuses without FORGEJO_URL and sends nothing"
  fi

  # FORGEJO_URL set: the request goes to the named host only.
  run "$script" env FORGEJO_URL=https://forge.test.invalid
  if [ -s "$log" ] && ! grep -q 'forge.test.invalid' "$log"; then
    echo "✗ $name: request went somewhere other than FORGEJO_URL: $(cat "$log")"
    fail=1
  elif grep -q 'git.b4mad' "$log" 2>/dev/null; then
    echo "✗ $name: request reached the upstream default host"
    fail=1
  else
    echo "✓ $name: with FORGEJO_URL set, requests go to that host"
  fi
done

# castra also refuses without a token, before any request.
run scripts/ci/check-castra-release-onboarding.sh env -u FORGEJO_TOKEN FORGEJO_URL=https://forge.test.invalid
if [ "$rc" -eq 0 ] || ! grep -q 'FORGEJO_TOKEN' "$tmp/out" || [ -s "$log" ]; then
  echo "✗ check-castra-release-onboarding.sh: FORGEJO_TOKEN unset was not refused cleanly"
  fail=1
else
  echo "✓ check-castra-release-onboarding.sh: refuses without FORGEJO_TOKEN and sends nothing"
fi

if [ "$fail" -ne 0 ]; then
  echo "check-forge-host-defaults: FAILED" >&2
  exit 1
fi
exit 0
