#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Bootstraps a freshly started Forgejo for the nightly live run: waits for it,
# finishes the install form (creates the admin account), mints an API token for
# that account and creates the repository test/e2e/attachments.sh writes to.
#
# The admin password is generated here and never leaves this script. The token
# and the coordinates the later steps need are appended to $BOOTSTRAP_ENV_FILE
# (under Actions, $GITHUB_ENV). Nothing is stored in the repository or in CI
# secrets; the instance is throwaway and dies with the job.
#
# Usage (inside the nightly job):
#   FORGEJO_URL=http://forgejo:3000 BOT_USER=ci-bot bash test/live/bootstrap.sh
#
# Requires: curl, jq, sed, od, head.
set -euo pipefail

fail() { echo "bootstrap: $*" >&2; exit 1; }

: "${FORGEJO_URL:?FORGEJO_URL required (e.g. http://forgejo:3000)}"
BOT_USER="${BOT_USER:-ci-bot}"
REPO="${REPO:-e2e-attachments}"
WAIT_TRIES="${BOOTSTRAP_WAIT_TRIES:-60}"
ENV_FILE="${BOOTSTRAP_ENV_FILE:-${GITHUB_ENV:-}}"
[[ -n "$ENV_FILE" ]] || fail "set BOOTSTRAP_ENV_FILE (or run under Actions, which sets GITHUB_ENV)"
command -v curl >/dev/null || fail "curl required"
command -v jq >/dev/null || fail "jq required"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
jar="$workdir/cookies"

# 1. Wait until the server answers.
tries=0
until curl -fsS -o /dev/null "$FORGEJO_URL/api/healthz"; do
    tries=$((tries + 1))
    [[ "$tries" -lt "$WAIT_TRIES" ]] || fail "Forgejo not healthy at $FORGEJO_URL after $tries tries"
    sleep 2
done

# Generated per run, held in a shell variable only.
pw="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"

# 2. Finish the install form. A CSRF token is read from the form first; an
#    instance that is already installed answers /install without one, and we
#    move on to the token step.
csrf="$(curl -sS -c "$jar" -b "$jar" "$FORGEJO_URL/install" \
    | sed -n 's/.*name="_csrf" value="\([^"]*\)".*/\1/p' | head -n1)"
if [[ -n "$csrf" ]]; then
    code="$(curl -sS -c "$jar" -b "$jar" -o /dev/null -w '%{http_code}' \
        --data-urlencode "_csrf=$csrf" \
        -d db_type=sqlite3 \
        -d db_path=/data/gitea/forgejo.db \
        -d app_name=forgejo-live \
        -d repo_root_path=/data/git/repositories \
        -d lfs_root_path=/data/git/lfs \
        -d log_root_path=/data/gitea/log \
        -d run_user=git \
        -d domain=forgejo \
        -d ssh_port=22 \
        -d http_port=3000 \
        --data-urlencode "app_url=$FORGEJO_URL/" \
        -d admin_name="$BOT_USER" \
        --data-urlencode "admin_passwd=$pw" \
        --data-urlencode "admin_confirm_passwd=$pw" \
        -d admin_email="$BOT_USER@example.com" \
        "$FORGEJO_URL/install")"
    [[ "$code" == 200 || "$code" == 302 ]] || fail "install form answered HTTP $code"
fi

# 3. Mint an API token for the admin account (basic auth, once).
resp="$(curl -sS -u "$BOT_USER:$pw" -H 'Content-Type: application/json' \
    -d '{"name":"nightly-live","scopes":["all"]}' \
    "$FORGEJO_URL/api/v1/users/$BOT_USER/tokens")"
token="$(jq -r '.sha1 // empty' <<<"$resp")"
[[ -n "$token" ]] || fail "token response had no sha1 (is the admin account present?)"

# 4. Create the test repository the e2e script writes issues into.
code="$(curl -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: token $token" -H 'Content-Type: application/json' \
    -d "{\"name\":\"$REPO\",\"auto_init\":true}" \
    "$FORGEJO_URL/api/v1/user/repos")"
[[ "$code" == 201 || "$code" == 409 ]] || fail "create repo $REPO answered HTTP $code"

# 5. Hand the coordinates to later steps. The token is masked in the log only.
echo "::add-mask::$token"
{
    echo "FORGEJO_URL=$FORGEJO_URL"
    echo "FORGEJO_ACCESS_TOKEN=$token"
    echo "OWNER=$BOT_USER"
    echo "REPO=$REPO"
} >>"$ENV_FILE"
echo "bootstrap: admin $BOT_USER and repo $REPO ready on $FORGEJO_URL"
