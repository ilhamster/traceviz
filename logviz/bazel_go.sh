#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-}"
if [[ -z "${cmd}" ]]; then
  echo "usage: $0 <build|test>" >&2
  exit 2
fi

root="${BUILD_WORKSPACE_DIRECTORY:-}"
if [[ -z "${root}" ]]; then
  echo "BUILD_WORKSPACE_DIRECTORY is not set" >&2
  exit 2
fi

cd "${root}/logviz"

case "${cmd}" in
  build)
    go build ./...
    ;;
  test)
    go test ./...
    ;;
  *)
    echo "unknown command: ${cmd}" >&2
    exit 2
    ;;
esac
