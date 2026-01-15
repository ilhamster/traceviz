#!/usr/bin/env bash
set -euo pipefail

server_bin=""
resource_root="${RESOURCE_ROOT:-}"
log_root="${LOG_ROOT:-}"

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

if [[ -z "${resource_root}" ]]; then
  resource_root="${root}/logviz/client/dist/client"
fi

resolve_runfile() {
  local path="$1"
  if [[ -n "${RUNFILES_DIR:-}" ]]; then
    echo "${RUNFILES_DIR}/${path}"
    return 0
  fi
  if [[ -n "${RUNFILES_MANIFEST_FILE:-}" ]]; then
    local entry
    entry="$(grep -m1 "^${path} " "${RUNFILES_MANIFEST_FILE}" || true)"
    if [[ -n "${entry}" ]]; then
      echo "${entry#* }"
      return 0
    fi
  fi
  local script_runfiles="${0}.runfiles"
  if [[ -d "${script_runfiles}" ]]; then
    echo "${script_runfiles}/${path}"
    return 0
  fi
  local script_manifest="${0}.runfiles_manifest"
  if [[ -f "${script_manifest}" ]]; then
    local entry
    entry="$(grep -m1 "^${path} " "${script_manifest}" || true)"
    if [[ -n "${entry}" ]]; then
      echo "${entry#* }"
      return 0
    fi
  fi
  return 1
}

if [[ -z "${server_bin}" ]]; then
  echo "missing --server-bin" >&2
  exit 2
fi

if [[ ! -x "${server_bin}" ]]; then
  server_bin="$(resolve_runfile "${server_bin}" || true)"
fi

if [[ ! -x "${server_bin}" ]]; then
  echo "server binary not found: ${server_bin}" >&2
  exit 1
fi

cd "${root}/logviz/client"
npm run build

args=(--resource_root "${resource_root}")
if [[ -n "${log_root}" ]]; then
  args+=(--log_root "${log_root}")
fi

exec "${server_bin}" "${args[@]}"
