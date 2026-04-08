#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${SMOKETEST_DEPS_LOG:-$ROOT_DIR/smoketest-deps.log}"
TMP_REPO="$(mktemp -d "${TMPDIR:-/tmp}/weaver-smoke-deps.XXXXXX")"
BINARY="${WEAVER_BIN:-$ROOT_DIR/bin/weaver}"

cleanup() {
  rm -rf "$TMP_REPO"
}

trap cleanup EXIT

mkdir -p "$(dirname "$LOG_FILE")"
exec > >(tee "$LOG_FILE") 2>&1

run() {
  echo
  echo "+ $*"
  "$@"
}

run_in() {
  local dir="$1"
  shift
  echo
  echo "+ (cd $dir && $*)"
  (
    cd "$dir"
    "$@"
  )
}

assert_eq() {
  local label="$1"
  local actual="$2"
  local expected="$3"

  echo
  echo "[deps-smoke] $label"
  echo "[deps-smoke] expected:"
  printf '%s\n' "$expected"
  echo "[deps-smoke] actual:"
  printf '%s\n' "$actual"

  if [[ "$actual" != "$expected" ]]; then
    echo "[deps-smoke] mismatch in $label"
    exit 1
  fi
}

echo "[deps-smoke] root: $ROOT_DIR"
echo "[deps-smoke] log: $LOG_FILE"
echo "[deps-smoke] temp repo: $TMP_REPO"

if [[ ! -x "$BINARY" ]]; then
  run make -C "$ROOT_DIR" build
fi

run_in "$TMP_REPO" git init -b main
run_in "$TMP_REPO" git config user.name "Weaver Deps Smoke"
run_in "$TMP_REPO" git config user.email "smoke@example.com"
run_in "$TMP_REPO" /bin/sh -c "echo base > README.md"
run_in "$TMP_REPO" git add README.md
run_in "$TMP_REPO" git commit -m "init"

run_in "$TMP_REPO" git branch feature-a
run_in "$TMP_REPO" git branch feature-b
run_in "$TMP_REPO" git branch feature-c
run_in "$TMP_REPO" git branch feature-d

run_in "$TMP_REPO" "$BINARY" init
run_in "$TMP_REPO" "$BINARY" stack feature-b --on feature-a
run_in "$TMP_REPO" "$BINARY" stack feature-c --on feature-b
run_in "$TMP_REPO" "$BINARY" stack feature-d --on main

deps_chain="$(cd "$TMP_REPO" && "$BINARY" deps feature-c)"
deps_tree="$(cd "$TMP_REPO" && "$BINARY" deps)"

expected_chain="main -> feature-a -> feature-b -> feature-c"
expected_tree=$'main\n+-- feature-a\n|   `-- feature-b\n|       `-- feature-c\n`-- feature-d'

assert_eq "deps chain for feature-c" "$deps_chain" "$expected_chain"
assert_eq "full deps tree" "$deps_tree" "$expected_tree"

echo
echo "[deps-smoke] success"
echo "[deps-smoke] temp repo cleaned on exit"
