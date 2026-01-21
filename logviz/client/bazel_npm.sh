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
  echo "logviz/client node_modules is missing; installing now." >&2
  (cd "${root}" && pnpm install)
fi

if [[ "${cmd}" == "build" ]] || [[ "${cmd}" == "test" ]] || [[ "${cmd}" == "start" ]] || [[ "${cmd}" == "watch" ]]; then
  if [[ ! -e "${root}/client/core/dist/core.d.ts" ]]; then
    echo "logviz/client ${cmd} requires client/core build; running it now." >&2
    (cd "${root}" && pnpm --filter ./client/core run build)
  fi
  if [[ ! -e "${root}/client/angular/dist/traceviz-angular/package.json" ]]; then
    echo "logviz/client ${cmd} requires client/angular build; running it now." >&2
    (cd "${root}" && pnpm --filter ./client/angular run build:lib)
  fi
fi

(cd "${root}" && pnpm --filter ./logviz/client run "${cmd}")
