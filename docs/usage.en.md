# Weaver Usage Guide

## Overview

Weaver helps you manage stacked branches locally. You declare branch dependencies once, then use those declarations for inspection, rebasing, composition, and export/import handoff.

## Setup

Build the binary:

```bash
make build
```

Initialize Weaver inside your Git repository:

```bash
./bin/weaver init
```

This creates:

- `.weaver.yaml`
- `.git/weaver/`

## Declare a Stack

Declare that `feature-b` depends on `feature-a`:

```bash
./bin/weaver stack feature-b --on feature-a
```

Add another branch on top:

```bash
./bin/weaver stack feature-c --on feature-b
```

Move a branch to a different parent:

```bash
./bin/weaver stack feature-c --on main
```

Remove a dependency declaration:

```bash
./bin/weaver unstack feature-c
```

## Inspect Dependencies

Show the full tree:

```bash
./bin/weaver deps
```

Show the chain for one branch:

```bash
./bin/weaver deps feature-c
```

Typical output:

```text
main -> feature-a -> feature-b -> feature-c
```

## Check Stack Health

Show the stack tree with health status:

```bash
./bin/weaver status
```

Possible health labels:

- `clean`
- `needs rebase`
- `conflict risk`

## Diagnose Local State

Run a read-only diagnostic pass:

```bash
./bin/weaver doctor
```

Get machine-readable output:

```bash
./bin/weaver doctor --json
```

`weaver doctor` checks initialization, config, declared branches, pending rebase state, and common Git state issues such as dirty working trees or in-progress operations.

## Update Local Branches From Upstream

Update explicit branches:

```bash
./bin/weaver update main feature-a feature-b
```

Update every tracked branch:

```bash
./bin/weaver update --all
```

Update a named group:

```bash
./bin/weaver update --group sprint-42
```

`weaver update` runs `git fetch --all`, then fast-forwards each selected local branch to its configured upstream ref. It stops if a branch has no upstream or cannot be fast-forwarded.

## Rebase a Stack

Rebase the full stack that leads to `feature-c`:

```bash
./bin/weaver sync feature-c
```

If you are already on the branch you want to sync:

```bash
./bin/weaver sync
```

If conflicts stop the process:

```bash
./bin/weaver continue
./bin/weaver abort
```

`continue` resumes after you resolve conflicts manually. `abort` stops the operation and returns to the original branch.

## Compose Branches

Dry-run a compose:

```bash
./bin/weaver compose feature-c --dry-run
```

Compose several branches together:

```bash
./bin/weaver compose feature-a feature-c feature-e
```

Compose all tracked branches:

```bash
./bin/weaver compose --all
```

Compose is ephemeral by default. It uses detached `HEAD`, performs the merges, and returns you to the original branch.

If you want a new integration branch created from the composed result, opt in explicitly:

```bash
./bin/weaver compose feature-b feature-d --base main --create integration
```

If you want an existing integration branch rebuilt from the clean base, opt in explicitly:

```bash
./bin/weaver compose feature-b feature-d --base main --update integration
```

The `--create` form creates `integration` from the composed commit and then restores your original branch.

The `--update` form starts from `main`, composes the requested branches, force-moves `integration` to that fresh result, and then restores your original branch.

## Manage Groups

Create a group:

```bash
./bin/weaver group create sprint-42 feature-a feature-b feature-c
```

Add more branches:

```bash
./bin/weaver group add sprint-42 feature-d feature-e
```

Remove branches from a group:

```bash
./bin/weaver group remove sprint-42 feature-c
```

Delete the group entirely:

```bash
./bin/weaver group remove sprint-42
```

List groups:

```bash
./bin/weaver group list
```

Compose a group:

```bash
./bin/weaver compose --group sprint-42
```

## Export and Import

Export local orchestration state:

```bash
./bin/weaver export > weaver-state.json
```

Import it into another clone:

```bash
./bin/weaver import weaver-state.json
```

## Smoke Test

Run the end-to-end verification script:

```bash
./smoketest.sh
```

It writes a step-by-step log to `smoketest.log`.
