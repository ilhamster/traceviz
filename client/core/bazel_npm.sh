#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-}"
if [[ -z "${cmd}" ]]; then
  echo "usage: $0 <npm-script>" >&2
  exit 2
fi

root="${BUILD_WORKSPACE_DIRECTORY:-}"
if [[ -z "${root}" ]]; then
  echo "BUILD_WORKSPACE_DIRECTORY is not set" >&2
  exit 2
fi

if [[ ! -d "${root}/node_modules" ]] || [[ "${cmd}" == "test" && ! -e "${root}/node_modules/ts-node" ]]; then
  echo "client/core dependencies missing; installing now." >&2
  (cd "${root}" && npm install)
fi

(cd "${root}" && npm --workspace client/core run "${cmd}")
