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

cd "${root}/client/core"

if [[ ! -d "node_modules" ]] || [[ "${cmd}" == "test" && ! -e "node_modules/ts-node" ]]; then
  echo "client/core dependencies missing; installing now." >&2
  npm install
fi

npm run "${cmd}"
