#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${root}"

echo "==> bazel: client/core tests"
bazel run //client/core:test

echo "==> bazel: client/angular headless tests"
bazel run //client/angular:test_headless

echo "==> bazel: logviz run"
bazel run //logviz:run
