#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${SMOKETEST_LOG:-$ROOT_DIR/smoketest.log}"
PRIMARY_REPO="$(mktemp -d "${TMPDIR:-/tmp}/weaver-smoke-primary.XXXXXX")"
IMPORT_REPO="$(mktemp -d "${TMPDIR:-/tmp}/weaver-smoke-import.XXXXXX")"
REMOTE_REPO="$(mktemp -d "${TMPDIR:-/tmp}/weaver-smoke-remote.XXXXXX")"
PUBLISH_REPO="$(mktemp -d "${TMPDIR:-/tmp}/weaver-smoke-publish.XXXXXX")"

cleanup() {
  rm -rf "$PRIMARY_REPO" "$IMPORT_REPO" "$REMOTE_REPO" "$PUBLISH_REPO"
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

echo "[smoke] root: $ROOT_DIR"
echo "[smoke] log: $LOG_FILE"
echo "[smoke] primary repo: $PRIMARY_REPO"
echo "[smoke] import repo: $IMPORT_REPO"
echo "[smoke] remote repo: $REMOTE_REPO"
echo "[smoke] publish repo: $PUBLISH_REPO"

run make -C "$ROOT_DIR" build

BINARY="$ROOT_DIR/bin/weaver"
STATE_FILE="$ROOT_DIR/smoketest-state.json"
INTEGRATION_FILE="$ROOT_DIR/smoketest-integration.json"
rm -f "$STATE_FILE"
rm -f "$INTEGRATION_FILE"

run_in "$PRIMARY_REPO" git init -b main
run_in "$PRIMARY_REPO" git config user.name "Weaver Smoke"
run_in "$PRIMARY_REPO" git config user.email "smoke@example.com"
run_in "$PRIMARY_REPO" /bin/sh -c "echo base > README.md"
run_in "$PRIMARY_REPO" git add README.md
run_in "$PRIMARY_REPO" git commit -m "init"

run_in "$PRIMARY_REPO" git checkout -b feature-a
run_in "$PRIMARY_REPO" /bin/sh -c "echo feature-a > feature-a.txt"
run_in "$PRIMARY_REPO" git add feature-a.txt
run_in "$PRIMARY_REPO" git commit -m "feature-a"

run_in "$PRIMARY_REPO" git checkout -b feature-b
run_in "$PRIMARY_REPO" /bin/sh -c "echo feature-b > feature-b.txt"
run_in "$PRIMARY_REPO" git add feature-b.txt
run_in "$PRIMARY_REPO" git commit -m "feature-b"

run_in "$PRIMARY_REPO" git checkout -b feature-c
run_in "$PRIMARY_REPO" /bin/sh -c "echo feature-c > feature-c.txt"
run_in "$PRIMARY_REPO" git add feature-c.txt
run_in "$PRIMARY_REPO" git commit -m "feature-c"

run_in "$PRIMARY_REPO" git checkout main
run_in "$PRIMARY_REPO" /bin/sh -c "echo main-update > main.txt"
run_in "$PRIMARY_REPO" git add main.txt
run_in "$PRIMARY_REPO" git commit -m "main-update"
run_in "$PRIMARY_REPO" git branch integration

run git init --bare "$REMOTE_REPO"
run_in "$PRIMARY_REPO" git remote add origin "$REMOTE_REPO"
run_in "$PRIMARY_REPO" git push -u origin main
run_in "$PRIMARY_REPO" git push -u origin feature-a
run_in "$PRIMARY_REPO" git push -u origin feature-b
run_in "$PRIMARY_REPO" git push -u origin feature-c
run_in "$PRIMARY_REPO" git push -u origin integration
run_in "$REMOTE_REPO" git symbolic-ref HEAD refs/heads/main
run git clone "$REMOTE_REPO" "$PUBLISH_REPO"
run_in "$PUBLISH_REPO" git config user.name "Weaver Smoke"
run_in "$PUBLISH_REPO" git config user.email "smoke@example.com"
run_in "$PUBLISH_REPO" git checkout main
run_in "$PUBLISH_REPO" /bin/sh -c "echo remote-main > remote-main.txt"
run_in "$PUBLISH_REPO" git add remote-main.txt
run_in "$PUBLISH_REPO" git commit -m "remote-main"
run_in "$PUBLISH_REPO" git push origin main
run_in "$PUBLISH_REPO" git checkout feature-a
run_in "$PUBLISH_REPO" /bin/sh -c "echo remote-feature-a >> feature-a.txt"
run_in "$PUBLISH_REPO" git add feature-a.txt
run_in "$PUBLISH_REPO" git commit -m "remote-feature-a"
run_in "$PUBLISH_REPO" git push origin feature-a

run_in "$PRIMARY_REPO" "$BINARY" init
run_in "$PRIMARY_REPO" "$BINARY" version
run_in "$PRIMARY_REPO" "$BINARY" stack feature-b --on feature-a
run_in "$PRIMARY_REPO" "$BINARY" stack feature-c --on feature-b
run_in "$PRIMARY_REPO" "$BINARY" integration save integration --base main feature-a feature-b feature-c
run_in "$PRIMARY_REPO" "$BINARY" integration show integration
run_in "$PRIMARY_REPO" "$BINARY" deps feature-c
run_in "$PRIMARY_REPO" "$BINARY" update main feature-a
run_in "$PRIMARY_REPO" "$BINARY" update --integration integration
main_rev="$(cd "$PRIMARY_REPO" && git rev-parse main)"
origin_main_rev="$(cd "$PRIMARY_REPO" && git rev-parse origin/main)"
feature_a_rev="$(cd "$PRIMARY_REPO" && git rev-parse feature-a)"
origin_feature_a_rev="$(cd "$PRIMARY_REPO" && git rev-parse origin/feature-a)"
if [[ "$main_rev" != "$origin_main_rev" ]]; then
  echo "[smoke] expected main to match origin/main after update"
  exit 1
fi
if [[ "$feature_a_rev" != "$origin_feature_a_rev" ]]; then
  echo "[smoke] expected feature-a to match origin/feature-a after update"
  exit 1
fi
run_in "$PRIMARY_REPO" "$BINARY" status
run_in "$PRIMARY_REPO" "$BINARY" doctor

run_in "$PRIMARY_REPO" "$BINARY" group create sprint-42 feature-a feature-c
run_in "$PRIMARY_REPO" "$BINARY" group add sprint-42 feature-b
run_in "$PRIMARY_REPO" "$BINARY" group list
run_in "$PRIMARY_REPO" "$BINARY" group remove sprint-42 feature-c
run_in "$PRIMARY_REPO" "$BINARY" group list

run_in "$PRIMARY_REPO" "$BINARY" compose feature-c --dry-run
run_in "$PRIMARY_REPO" "$BINARY" compose --integration integration --dry-run
run_in "$PRIMARY_REPO" "$BINARY" compose feature-c --base main --create integration-preview --dry-run
run_in "$PRIMARY_REPO" "$BINARY" compose feature-c --base main --update integration-preview --dry-run
run_in "$PRIMARY_REPO" "$BINARY" compose --group sprint-42 --dry-run
run_in "$PRIMARY_REPO" "$BINARY" compose --all --dry-run

run_in "$PRIMARY_REPO" git checkout feature-c
run_in "$PRIMARY_REPO" "$BINARY" sync feature-c
if [[ -f "$PRIMARY_REPO/.git/weaver/rebase-state.yaml" ]]; then
  echo "[smoke] expected rebase state to be cleared"
  exit 1
fi
run_in "$PRIMARY_REPO" "$BINARY" status

run_in "$PRIMARY_REPO" "$BINARY" compose --group sprint-42
run_in "$PRIMARY_REPO" git branch --show-current
run_in "$PRIMARY_REPO" "$BINARY" compose feature-c --base main --create integration-preview
if ! (cd "$PRIMARY_REPO" && git show-ref --verify --quiet refs/heads/integration-preview); then
  echo "[smoke] expected integration-preview branch to be created"
  exit 1
fi
run_in "$PRIMARY_REPO" git checkout integration-preview
run_in "$PRIMARY_REPO" /bin/sh -c "echo stale > integration-preview-only.txt"
run_in "$PRIMARY_REPO" git add integration-preview-only.txt
run_in "$PRIMARY_REPO" git commit -m "integration-preview-drift"
run_in "$PRIMARY_REPO" git checkout feature-c
integration_preview_before="$(cd "$PRIMARY_REPO" && git rev-parse integration-preview)"
run_in "$PRIMARY_REPO" "$BINARY" compose --integration integration --update integration-preview
integration_preview_after="$(cd "$PRIMARY_REPO" && git rev-parse integration-preview)"
if [[ "$integration_preview_before" == "$integration_preview_after" ]]; then
  echo "[smoke] expected integration-preview branch to change after update"
  exit 1
fi
if (cd "$PRIMARY_REPO" && git cat-file -e integration-preview:integration-preview-only.txt 2>/dev/null); then
  echo "[smoke] expected update to drop integration-preview-only.txt"
  exit 1
fi
if [[ "$(cd "$PRIMARY_REPO" && git show integration-preview:feature-c.txt | tr -d '\n')" != "feature-c" ]]; then
  echo "[smoke] expected integration-preview to include feature-c after update"
  exit 1
fi

run_in "$PRIMARY_REPO" /bin/sh -c "\"$BINARY\" integration export integration --json > \"$INTEGRATION_FILE\""
run_in "$PRIMARY_REPO" cat "$INTEGRATION_FILE"
run_in "$PRIMARY_REPO" /bin/sh -c "\"$BINARY\" export > \"$STATE_FILE\""
run_in "$PRIMARY_REPO" cat "$STATE_FILE"

run_in "$IMPORT_REPO" git init -b main
run_in "$IMPORT_REPO" git config user.name "Weaver Smoke"
run_in "$IMPORT_REPO" git config user.email "smoke@example.com"
run_in "$IMPORT_REPO" /bin/sh -c "echo imported > README.md"
run_in "$IMPORT_REPO" git add README.md
run_in "$IMPORT_REPO" git commit -m "init"
run_in "$IMPORT_REPO" "$BINARY" init
run_in "$IMPORT_REPO" "$BINARY" integration import "$INTEGRATION_FILE"
run_in "$IMPORT_REPO" "$BINARY" integration show integration
run_in "$IMPORT_REPO" "$BINARY" import "$STATE_FILE"
run_in "$IMPORT_REPO" "$BINARY" deps feature-c
run_in "$IMPORT_REPO" "$BINARY" integration show integration
run_in "$IMPORT_REPO" "$BINARY" group list

rm -f "$STATE_FILE"
rm -f "$INTEGRATION_FILE"

echo
echo "[smoke] success"
echo "[smoke] temp repos cleaned on exit"
