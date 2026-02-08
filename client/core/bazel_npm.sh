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

if [[ ! -d "${traceviz_root}/node_modules" ]] || [[ "${cmd}" == "test" && ! -e "${traceviz_root}/node_modules/ts-node" ]]; then
  echo "client/core dependencies missing; installing now." >&2
  (cd "${traceviz_root}" && pnpm install --no-frozen-lockfile)
fi

(cd "${traceviz_root}" && pnpm --filter ./client/core run "${cmd}")
