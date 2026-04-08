# Weaver Usage Guide

## Overview

Weaver helps you manage stacked branches locally. You declare branch dependencies once, then use those declarations for inspection, rebasing, composition, and export/import handoff.

## Setup

Install the latest tagged release:

```bash
go install github.com/lutefd/weaver@latest
weaver version
```

Initialize Weaver inside your Git repository:

```bash
weaver init
```

This creates:

- `.weaver.yaml`
- `.git/weaver/`

## Declare a Stack

Declare that `feature-b` depends on `feature-a`:

```bash
weaver stack feature-b --on feature-a
```

Add another branch on top:

```bash
weaver stack feature-c --on feature-b
```

Move a branch to a different parent:

```bash
weaver stack feature-c --on main
```

Remove a dependency declaration:

```bash
weaver unstack feature-c
```

## Inspect Dependencies

Show the full tree:

```bash
weaver deps
```

Show the chain for one branch:

```bash
weaver deps feature-c
```

Typical output:

```text
main -> feature-a -> feature-b -> feature-c
```

## Check Stack Health

Show the stack tree with health status:

```bash
weaver status
```

Possible health labels:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnose Local State

Run a read-only diagnostic pass:

```bash
weaver doctor
```

Get machine-readable output:

```bash
weaver doctor --json
```

`weaver doctor` checks initialization, config, declared branches, pending rebase state, and common Git state issues such as dirty working trees or in-progress operations.

## Update Local Branches From Upstream

Update explicit branches:

```bash
weaver update main feature-a feature-b
```

Update every tracked branch:

```bash
weaver update --all
```

Update a named group:

```bash
weaver update --group sprint-42
```

Update every branch tracked by a saved integration strategy:

```bash
weaver update --integration integration
```

`weaver update` runs `git fetch --all`, then fast-forwards each selected local branch to its configured upstream ref. It stops if a branch has no upstream or cannot be fast-forwarded.

## Rebase a Stack

Rebase the full stack that leads to `feature-c`:

```bash
weaver sync feature-c
```

If you are already on the branch you want to sync:

```bash
weaver sync
```

If the branches already have open PRs and you want to preserve their history, merge each parent into the stack instead. This fast-forwards when possible and otherwise records the normal merge commit:

```bash
weaver sync feature-c --merge
```

If conflicts stop the process:

```bash
weaver continue
weaver abort
```

`continue` resumes after you resolve conflicts manually. `abort` stops the operation and returns to the original branch. The same commands work for both rebase-based and merge-based stack sync.

## Compose Branches

Dry-run a compose:

```bash
weaver compose feature-c --dry-run
```

Compose several branches together:

```bash
weaver compose feature-a feature-c feature-e
```

Compose all tracked branches:

```bash
weaver compose --all
```

Skip a problematic branch but keep the rest of the compose:

```bash
weaver compose --integration integration --create integration-preview --skip feature-debug-search-api-curl
```

Compose is ephemeral by default. It uses detached `HEAD`, performs the merges, and returns you to the original branch.

If you want a new integration branch created from the composed result, opt in explicitly:

```bash
weaver compose feature-b feature-d --base main --create integration
```

If you want an existing integration branch rebuilt from the clean base, opt in explicitly:

```bash
weaver compose feature-b feature-d --base main --update integration
```

The `--create` form creates `integration` from the composed commit and then restores your original branch.

The `--update` form starts from `main`, composes the requested branches, force-moves `integration` to that fresh result, and then restores your original branch.

If compose hits a conflict, Weaver reports the branch that failed and the conflicted files.

If you did not already pass `--skip`, Weaver prompts you to skip the failing branch or abort the compose.

If one branch is heavily diverged and keeps breaking a large compose, it is often better to remove that branch from the compose or saved integration, repair it first, and then merge it manually onto the branch produced by `--create` or `--update` before adding it back.

If you already saved a reusable strategy, compose from it directly:

```bash
weaver compose --integration integration --update integration
```

When `--integration` is used, Weaver takes the base and selected branches from the saved strategy.

## Manage Saved Integrations

Save or update a strategy:

```bash
weaver integration save integration --base main feature-a feature-b feature-c
```

Show it:

```bash
weaver integration show integration
```

Diagnose it:

```bash
weaver integration doctor integration
weaver integration doctor integration --json
```

List saved strategies:

```bash
weaver integration list
```

Remove one:

```bash
weaver integration remove integration
```

Export one strategy as JSON:

```bash
weaver integration export integration --json > integration.json
```

Import it into another clone:

```bash
weaver integration import integration.json
```

## Manage Groups

Create a group:

```bash
weaver group create sprint-42 feature-a feature-b feature-c
```

Add more branches:

```bash
weaver group add sprint-42 feature-d feature-e
```

Remove branches from a group:

```bash
weaver group remove sprint-42 feature-c
```

Delete the group entirely:

```bash
weaver group remove sprint-42
```

List groups:

```bash
weaver group list
```

Compose a group:

```bash
weaver compose --group sprint-42
```

## Export and Import

Export local orchestration state, including saved integrations:

```bash
weaver export > weaver-state.json
```

Import it into another clone:

```bash
weaver import weaver-state.json
```

## Smoke Test

Run the end-to-end verification script:

```bash
./smoketest.sh
```

It writes a step-by-step log to `smoketest.log`.
