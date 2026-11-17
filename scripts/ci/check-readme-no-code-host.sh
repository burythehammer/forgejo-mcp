# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-readme-no-code-host.sh — README.md names no code host and links to none.
#
# The project is mirrored on more than one forge, so the README speaks
# generically ("this project", "the repository") and keeps links to repo files
# relative. The wording rule (spec #362, decision 1) cannot quietly regress
# without a check, so this one fails the build when a host name comes back.
#
# The rule, exactly:
#
#   1. Forbidden: any name in HOSTS below, matched case-insensitively as a
#      substring anywhere in README.md — prose, link text, or URL. The list is
#      the code hosts this project has lived on or is mirrored to (go.mod's
#      module host, the `origin`/`github` remotes, the `upstream` remote), plus
#      the chat and forge names a README is likely to drift back to.
#   2. The one allowed exception: the module path from go.mod, but only on a
#      line that contains `go install`. The install line carries it as is.
#      The path is read from go.mod, so a module rename moves the exception
#      with it. Anywhere else the module path is forbidden like any other host.
#   3. Project names are not hosts. `owner/repo` paths, bare owner names and
#      the upstream project name (`b4mad/forgejo-mcp` in the Origin line) pass:
#      the list matches host names, not repository paths. A host written in
#      a URL is caught by its host name, so a URL needs no separate rule.
#   4. Relative links (docs/…, ../…) pass. The script does not check that they
#      resolve; that is a separate step done by hand when the README changes.
#
# Limit: the list names known hosts. A host that is not on it passes, so a new
# mirror has to be added to HOSTS in the same change that starts naming it.
#
# Exit 0 when the README names no code host. Exit 1 listing each offending line.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal CI image.

set -eu

# awk's tolower() and comparisons are byte-wise under the C locale.
export LC_ALL=C

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

README=README.md
[ -f "$README" ] || { echo "check-readme-no-code-host: $README not found" >&2; exit 1; }

MODULE="$(sed -n 's/^module //p' go.mod)"
[ -n "$MODULE" ] || { echo "check-readme-no-code-host: no module line in go.mod" >&2; exit 1; }

# Lowercase ERE alternatives. Dots are written as [.] so no escapes reach awk.
HOSTS='codeberg|github|gitlab|bitbucket|sourcehut|sr[.]ht|gitea|radicle|forgejo[.]burythehammer|git[.]b4mad|b4mad[.]industries|matrix[.](org|to)|git@|ssh://|git://'

# On a go install line, blank the module path (fixed string, not a regex) before
# matching. Report the original line so the reader sees what to fix.
hits="$(awk -v m="$MODULE" -v re="$HOSTS" '
  {
    s = $0
    if (s ~ /go install/) {
      i = index(s, m)
      if (i > 0) s = substr(s, 1, i - 1) substr(s, i + length(m))
    }
    if (tolower(s) ~ re) print NR ": " $0
  }' "$README")"

if [ -n "$hits" ]; then
  echo "$hits"
  echo "" >&2
  echo "check-readme-no-code-host: $README names a code host or links to one." >&2
  echo "Use generic wording and relative links. The only exception is the module" >&2
  echo "path on a go install line ($MODULE)." >&2
  exit 1
fi

echo "$README names no code host."
