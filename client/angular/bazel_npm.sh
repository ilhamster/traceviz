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

if [[ ! -d "${root}/node_modules" ]] || [[ "${cmd}" == "build:lib" && ! -e "${root}/node_modules/.bin/ng-packagr" ]]; then
  echo "client/angular dependencies missing; installing now." >&2
  (cd "${root}" && npm install)
fi

if [[ "${cmd}" == "build:lib" ]] && [[ ! -e "${root}/client/core/dist/core.d.ts" ]]; then
  echo "client/angular build:lib requires client/core build; running it now." >&2
  (cd "${root}" && npm --workspace client/core run build)
fi

if [[ "${cmd}" == "test:headless" ]] && [[ ! -e "${root}/client/core/dist/core.d.ts" ]]; then
  echo "client/angular test:headless requires client/core build; running it now." >&2
  (cd "${root}" && npm --workspace client/core run build)
fi

(cd "${root}" && npm --workspace client/angular run "${cmd}")
