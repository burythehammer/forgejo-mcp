#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Offline test for test/live/bootstrap.sh. A fake curl on PATH answers the
# Forgejo endpoints the script calls, so no server or token is needed.
# Run by hand: bash test/live/bootstrap_test.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/bootstrap.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

cat >"$work/bin/curl" <<'EOF'
#!/usr/bin/env bash
# Fake curl: answers by URL (the last argument) and records every call.
url="${*: -1}"
printf '%s\n' "$*" >>"$FAKE_LOG"
case "$url" in
    */api/healthz) exit "${FAKE_HEALTH:-0}" ;;
    */install)
        if [[ "$*" == *"_csrf="* ]]; then
            printf '%s' "${FAKE_INSTALL_CODE:-302}"
        else
            printf '<input type="hidden" name="_csrf" value="csrf-123">'
        fi
        ;;
    */api/v1/users/*/tokens)
        if [[ -n "${FAKE_TOKEN_JSON+x}" ]]; then
            printf '%s' "$FAKE_TOKEN_JSON"
        else
            printf '%s' '{"sha1":"tok-abc"}'
        fi
        ;;
    */api/v1/user/repos) printf '%s' "${FAKE_REPO_CODE:-201}" ;;
    *) echo "fake curl: unexpected $url" >&2; exit 9 ;;
esac
EOF
chmod +x "$work/bin/curl"

pass=0
failures=0

# run NAME EXPECT(ok|fail) [VAR=value ...]: runs the script with the fake curl.
run() {
    local name="$1" expect="$2"
    shift 2
    local envfile="$work/$name.env" log="$work/$name.log" rc=0
    : >"$log"
    env -i PATH="$work/bin:/usr/bin:/bin" FAKE_LOG="$log" \
        FORGEJO_URL=http://forgejo.test:3000 BOOTSTRAP_ENV_FILE="$envfile" \
        BOOTSTRAP_WAIT_TRIES=1 "$@" \
        bash "$script" >"$work/$name.out" 2>&1 || rc=$?
    if [[ "$expect" == ok && "$rc" -ne 0 ]] || [[ "$expect" == fail && "$rc" -eq 0 ]]; then
        echo "FAIL $name: expected $expect, exit $rc"
        sed 's/^/    /' "$work/$name.out"
        failures=$((failures + 1))
        return
    fi
    pass=$((pass + 1))
    echo "ok   $name"
}

# Happy path: the env file gets the token and coordinates, and nothing else.
run happy ok
grep -qx 'FORGEJO_ACCESS_TOKEN=tok-abc' "$work/happy.env" || { echo "FAIL happy: token not written"; failures=$((failures + 1)); }
grep -qx 'OWNER=ci-bot' "$work/happy.env" || { echo "FAIL happy: OWNER not written"; failures=$((failures + 1)); }
grep -qx 'REPO=e2e-attachments' "$work/happy.env" || { echo "FAIL happy: REPO not written"; failures=$((failures + 1)); }
[[ "$(wc -l <"$work/happy.env")" -eq 4 ]] || { echo "FAIL happy: env file has extra lines"; failures=$((failures + 1)); }

# The generated password must not reach the env file.
if grep -q 'admin_passwd' "$work/happy.env"; then
    echo "FAIL happy: password leaked into env file"; failures=$((failures + 1))
fi

# Token response without sha1 must fail, and must not write a token.
run no_token fail FAKE_TOKEN_JSON='{"message":"user does not exist"}'
if grep -q FORGEJO_ACCESS_TOKEN "$work/no_token.env" 2>/dev/null; then
    echo "FAIL no_token: token written despite failure"; failures=$((failures + 1))
fi

# Install form refused: fail before minting a token.
run install_refused fail FAKE_INSTALL_CODE=500
if grep -q '/tokens' "$work/install_refused.log"; then
    echo "FAIL install_refused: token call made after failed install"; failures=$((failures + 1))
fi

# Repo creation refused: fail.
run repo_refused fail FAKE_REPO_CODE=500

# Repo already present (409) is fine on a rerun.
run repo_exists ok FAKE_REPO_CODE=409

# No env file destination: fail before any network call.
envless_log="$work/envless.log"; : >"$envless_log"
if env -i PATH="$work/bin:/usr/bin:/bin" FAKE_LOG="$envless_log" \
    FORGEJO_URL=http://forgejo.test:3000 bash "$script" >/dev/null 2>&1; then
    echo "FAIL envless: script ran without an env file"; failures=$((failures + 1))
elif [[ -s "$envless_log" ]]; then
    echo "FAIL envless: network call made before the env file check"; failures=$((failures + 1))
else
    pass=$((pass + 1)); echo "ok   envless"
fi

echo "--"
echo "passed $pass, failed $failures"
[[ "$failures" -eq 0 ]]
