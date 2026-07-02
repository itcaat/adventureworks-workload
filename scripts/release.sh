#!/usr/bin/env bash
set -euo pipefail

remote="${RELEASE_REMOTE:-origin}"

git fetch --tags "$remote"

if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
  echo "error: HEAD already has tag $(git describe --tags --exact-match HEAD)" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree has uncommitted changes" >&2
  exit 1
fi

latest=$(
  git ls-remote --tags "$remote" \
    | awk '/refs\/tags\/v[0-9]/{sub(/.*\//, ""); if ($0 !~ /\^{}$/) print}' \
    | sort -V \
    | tail -1
)

if [[ -z "$latest" ]]; then
  new_tag=v0.0.1
else
  version=${latest#v}
  IFS=. read -r major minor patch _ <<<"$version"
  if [[ -z "${major:-}" || -z "${minor:-}" || -z "${patch:-}" ]]; then
    echo "error: latest tag $latest is not semver vMAJOR.MINOR.PATCH" >&2
    exit 1
  fi
  new_tag="v${major}.${minor}.$((patch + 1))"
fi

branch=$(git rev-parse --abbrev-ref HEAD)
if upstream=$(git rev-parse --abbrev-ref '@{u}' 2>/dev/null); then
  git fetch "$remote" "${upstream#*/}"
  local_head=$(git rev-parse HEAD)
  remote_head=$(git rev-parse "$upstream")
  if [[ "$local_head" != "$remote_head" ]]; then
    echo "pushing $branch to $remote before tagging"
    git push "$remote" HEAD:"${upstream#*/}"
  fi
else
  echo "warning: no upstream branch; pushing $branch to $remote"
  git push "$remote" HEAD:"$branch"
fi

echo "latest remote tag: ${latest:-<none>}"
echo "creating tag:        $new_tag"

git tag -a "$new_tag" -m "Release $new_tag"
git push "$remote" "$new_tag"

echo "pushed $new_tag — GitHub Actions release workflow should start"
