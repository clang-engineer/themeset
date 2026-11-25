#!/bin/sh
set -eu

version=${1:?Usage: scripts/package.sh VERSION [REPOSITORY]}
repository=${2:-${GITHUB_REPOSITORY:-}}
case "$version" in
  v[0-9]*) ;;
  *)
    printf '%s\n' 'version must start with v and a digit' >&2
    exit 2
    ;;
esac
case "$version" in *[!a-zA-Z0-9._-]*) exit 2 ;; esac
name=themeset
mkdir -p dist
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  os=${target%/*}
  arch=${target#*/}
  stage=$(mktemp -d)
  trap 'rm -rf "$stage"' EXIT HUP INT TERM
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X github.com/clang-engineer/themeset/internal/app.Version=$version" \
    -o "$stage/$name" ./cmd/themeset
  cp LICENSE README.md "$stage/"
  if [ -f THIRD_PARTY_NOTICES.md ]; then cp THIRD_PARTY_NOTICES.md "$stage/"; fi
  mkdir "$stage/licenses"
  cp licenses/* internal/adapter/assets/*.LICENSE "$stage/licenses/"
  archive="$name-${version#v}-$os-$arch.tar.gz"
  COPYFILE_DISABLE=1 tar -czf "dist/$archive" -C "$stage" .
  rm -rf "$stage"
  trap - EXIT HUP INT TERM
done
ruby scripts/formula.rb "$version" "$repository"
