#!/usr/bin/env bash
set -euo pipefail

root="${BUILD_WORKSPACE_DIRECTORY:-}"
if [[ -z "${root}" ]]; then
  echo "BUILD_WORKSPACE_DIRECTORY is not set" >&2
  exit 2
fi

resolve_runfile() {
  local path="$1"
  if [[ -n "${RUNFILES_DIR:-}" ]]; then
    local candidate="${RUNFILES_DIR}/${path}"
    if [[ -e "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  fi
  if [[ -n "${RUNFILES_MANIFEST_FILE:-}" ]]; then
    local entry
    entry="$(grep -m1 "^${path} " "${RUNFILES_MANIFEST_FILE}" || true)"
    if [[ -n "${entry}" ]]; then
      local resolved="${entry#* }"
      if [[ -e "${resolved}" ]]; then
        echo "${resolved}"
        return 0
      fi
    fi
  fi
  local script_runfiles="${0}.runfiles"
  if [[ -d "${script_runfiles}" ]]; then
    local candidate="${script_runfiles}/${path}"
    if [[ -e "${candidate}" ]]; then
      echo "${candidate}"
      return 0
    fi
  fi
  local script_manifest="${0}.runfiles_manifest"
  if [[ -f "${script_manifest}" ]]; then
    local entry
    entry="$(grep -m1 "^${path} " "${script_manifest}" || true)"
    if [[ -n "${entry}" ]]; then
      local resolved="${entry#* }"
      if [[ -e "${resolved}" ]]; then
        echo "${resolved}"
        return 0
      fi
    fi
  fi
  return 1
}

traceviz_pkg_json="$(resolve_runfile "traceviz+/package.json" || true)"
if [[ -z "${traceviz_pkg_json}" ]]; then
  traceviz_root="${root}"
else
  traceviz_root="$(cd "$(dirname "${traceviz_pkg_json}")" && pwd)"
fi

export PNPM_WORKSPACE_DIR="${PNPM_WORKSPACE_DIR:-${traceviz_root}}"

if [[ ! -d "${traceviz_root}/node_modules" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/jasmine" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/@traceviz/client-core/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/@mantine/core/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/@testing-library/react/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/jsdom/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/react/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/react-dom/package.json" ]] || \
   [[ ! -e "${traceviz_root}/node_modules/rxjs/package.json" ]]; then
  echo "traceviz dependencies missing; installing now." >&2
  (cd "${traceviz_root}" && CI=true pnpm install --no-frozen-lockfile)
fi

# The client-core package exports from dist/, so ensure it is built.
if [[ ! -e "${traceviz_root}/client/core/dist/core.js" ]]; then
  echo "traceviz client/core build required; running it now." >&2
  (cd "${traceviz_root}" && pnpm --filter ./client/core run build)
fi

(cd "${traceviz_root}" && TS_NODE_PROJECT=client/react/tsconfig.json NODE_OPTIONS="--enable-source-maps --loader=ts-node/esm" pnpm exec jasmine --config=client/react/spec/support/jasmine.json)
