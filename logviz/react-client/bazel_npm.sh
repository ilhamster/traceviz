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

if [[ ! -d "${root}/node_modules" ]]; then
  echo "logviz/react-client node_modules is missing; installing now." >&2
  (cd "${root}" && pnpm install)
fi

if [[ "${cmd}" == "build" ]] || [[ "${cmd}" == "test" ]] || [[ "${cmd}" == "dev" ]] || [[ "${cmd}" == "preview" ]]; then
  if [[ ! -e "${root}/client/core/dist/core.js" ]]; then
    echo "logviz/react-client ${cmd} requires client/core build; running it now." >&2
    (cd "${root}" && pnpm --filter ./client/core run build)
  fi
fi

(cd "${root}" && pnpm --filter ./logviz/react-client run "${cmd}")
