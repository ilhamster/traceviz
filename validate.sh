#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${root}"

reset_npm() {
  pnpm run reset
}

reset_bazel() {
  bazel clean --expunge
  rm -rf "${root}/.aspect"
}

section() {
  echo
  echo "==> $*"
}

section "pnpm: reset"
reset_npm

section "pnpm: build + test + logviz build (no run)"
pnpm run ibt
pnpm run ib:logviz

section "bazel: initial clean"
reset_bazel

section "bazel: client/core build"
reset_bazel
bazel build //client/core:build

section "bazel: client/core tests"
reset_bazel
bazel run //client/core:test

section "bazel: client/angular build_lib"
reset_bazel
bazel run //client/angular:build_lib

section "bazel: client/angular headless tests"
reset_bazel
bazel run //client/angular:test_headless

section "bazel: logviz build (go + client bundle)"
reset_bazel
bazel run //logviz:build

section "pnpm: final reset"
reset_npm

section "bazel: final clean"
reset_bazel
