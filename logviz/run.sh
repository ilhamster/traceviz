#!/usr/bin/env bash
set -euo pipefail

server_bin=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-bin)
      server_bin="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

root="${BUILD_WORKSPACE_DIRECTORY:-}"
if [[ -z "${root}" ]]; then
  echo "BUILD_WORKSPACE_DIRECTORY is not set" >&2
  exit 2
fi

if [[ -z "${server_bin}" ]]; then
  echo "missing --server-bin" >&2
  exit 2
fi

"${root}/logviz/client/bazel_npm.sh" build
SKIP_CLIENT_BUILD=1 "${root}/logviz/run_server.sh" --server-bin "${server_bin}"
