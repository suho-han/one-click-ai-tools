#!/usr/bin/env bash
# Computes the next release tag from Conventional Commits messages since the
# last release tag. Prints the version only (e.g. "v0.1.3") on stdout;
# everything else (baseline, commit breakdown, bump reason) goes to stderr.
#
# Baseline detection:
#   Uses the most recently *created* vX.Y.Z tag, not the highest by semver
#   sort. This repo's tag history has a deliberate version reset (distribution
#   moved from npm+GitHub to GitHub-Releases-only, and versioning restarted at
#   0.1.x) so v0.4.19 still semver-sorts above v0.1.3 even though it's not the
#   real latest release. See docs/release-checklist.md.
#
# Bump rule (Conventional Commits, 0.x-aware):
#   major == 0:
#     any "!" or "BREAKING CHANGE" commit -> bump MINOR (0.x has no stable
#       public API yet, so breaking changes don't get a major bump)
#     any "feat" commit                   -> bump PATCH
#     any "fix"/"perf" commit (no feat)   -> bump PATCH
#   major >= 1:
#     any "!" or "BREAKING CHANGE" commit -> bump MAJOR
#     any "feat" commit                   -> bump MINOR
#     any "fix"/"perf" commit (no feat)   -> bump PATCH
#   In both cases: if every commit since the baseline is something else
#   (chore/docs/style/refactor/test/ci/build/...), there is nothing
#   release-worthy -> the script errors out instead of bumping. This is what
#   makes `scripts/release-package.sh auto` safe to wire into CI on every
#   push to main: a docs typo fix doesn't cut a public release.
#
# Usage: bash scripts/next-version.sh
#        VERSION="$(bash scripts/next-version.sh)"
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BASELINE="$(git for-each-ref --sort=-creatordate --format '%(refname:short)' refs/tags \
  | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)"

if [[ -z "$BASELINE" ]]; then
  echo "ERROR: no existing vX.Y.Z tag found; pick a starting version manually" >&2
  exit 1
fi

BASE_VERSION="${BASELINE#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$BASE_VERSION"

commits="$(git log "${BASELINE}..HEAD" --format='%H')"
if [[ -z "$commits" ]]; then
  echo "ERROR: no commits since ${BASELINE}; nothing to release" >&2
  exit 1
fi

n_breaking=0
n_feat=0
n_fix=0
n_other=0

# NOTE: these patterns must stay in variables. Writing the escaped parens
# literally inside `[[ ... =~ ... ]]` makes bash's own command parser choke
# with "parentheses not balanced" before the regex engine ever sees it.
breaking_pat='^[a-zA-Z]+(\([^)]*\))?!: '
feat_pat='^feat(\([^)]*\))?: '
fix_pat='^(fix|perf)(\([^)]*\))?: '

while IFS= read -r sha; do
  [[ -z "$sha" ]] && continue
  subject="$(git show -s --format='%s' "$sha")"
  body="$(git show -s --format='%b' "$sha")"

  if [[ "$subject" =~ $breaking_pat ]] || [[ "$body" == *"BREAKING CHANGE"* ]]; then
    n_breaking=$((n_breaking + 1))
  elif [[ "$subject" =~ $feat_pat ]]; then
    n_feat=$((n_feat + 1))
  elif [[ "$subject" =~ $fix_pat ]]; then
    n_fix=$((n_fix + 1))
  else
    n_other=$((n_other + 1))
  fi
done <<< "$commits"

reason=""
releasable=1
if [[ "$MAJOR" -eq 0 ]]; then
  if [[ "$n_breaking" -gt 0 ]]; then
    MINOR=$((MINOR + 1)); PATCH=0
    reason="breaking change (0.x: bumps minor, not major)"
  elif [[ "$n_feat" -gt 0 ]]; then
    PATCH=$((PATCH + 1))
    reason="feat (0.x: bumps patch, not minor)"
  elif [[ "$n_fix" -gt 0 ]]; then
    PATCH=$((PATCH + 1))
    reason="fix/perf"
  else
    releasable=0
  fi
else
  if [[ "$n_breaking" -gt 0 ]]; then
    MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0
    reason="breaking change"
  elif [[ "$n_feat" -gt 0 ]]; then
    MINOR=$((MINOR + 1)); PATCH=0
    reason="feat"
  elif [[ "$n_fix" -gt 0 ]]; then
    PATCH=$((PATCH + 1))
    reason="fix/perf"
  else
    releasable=0
  fi
fi

if [[ "$releasable" -eq 0 ]]; then
  echo "baseline: ${BASELINE}" >&2
  echo "commits since baseline: breaking=${n_breaking} feat=${n_feat} fix/perf=${n_fix} other=${n_other}" >&2
  echo "ERROR: no feat/fix/perf/breaking commits since ${BASELINE} (only chore/docs/refactor/... found); nothing release-worthy" >&2
  exit 1
fi

NEXT="v${MAJOR}.${MINOR}.${PATCH}"

if git rev-parse -q --verify "refs/tags/${NEXT}" >/dev/null; then
  echo "ERROR: computed tag ${NEXT} already exists locally" >&2
  exit 1
fi
if command -v gh >/dev/null 2>&1; then
  if gh release view "$NEXT" >/dev/null 2>&1; then
    echo "ERROR: computed tag ${NEXT} is already a published GitHub release" >&2
    exit 1
  fi
fi

echo "baseline: ${BASELINE}" >&2
echo "commits since baseline: breaking=${n_breaking} feat=${n_feat} fix/perf=${n_fix} other=${n_other}" >&2
echo "bump reason: ${reason}" >&2
echo "next: ${NEXT}" >&2

echo "$NEXT"
