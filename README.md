# Weaver

Weaver is a local-first Go CLI for managing stacked Git branches without requiring GitHub metadata, PR conventions, or external services.

It stores stack relationships in `.git/weaver/`, keeps them out of the committed tree, and uses regular Git commands under the hood so the workflow stays inspectable.

## What It Does

- Declares branch dependencies locally with `weaver stack`.
- Resolves stacks as a DAG built from `.git/weaver/deps.yaml`.
- Shows stack structure and health with `weaver deps` and `weaver status`.
- Diagnoses broken local state with `weaver doctor`.
- Fetches upstream refs and fast-forwards local branches with `weaver update`.
- Rebases a stack in dependency order with crash-safe resume state.
- Composes multiple branches into an ephemeral integration state.
- Saves reusable integration strategies in `.git/weaver/integrations.yaml`.
- Manages named compose groups in `.git/weaver/groups.yaml`.
- Exports and imports local orchestration state as JSON.

## Current Commands

```text
weaver init
weaver version
weaver stack <branch> --on <parent>
weaver unstack <branch>
weaver deps [branch]
weaver status
weaver doctor [--json]
weaver update [branch...] [--group NAME | --integration NAME | --all]
weaver sync [branch]
weaver continue
weaver abort
weaver compose [branch...] [--group NAME | --integration NAME | --all] [--base BRANCH] [--create BRANCH | --update BRANCH] [--dry-run]
weaver integration save <name> <branch...> [--base BRANCH]
weaver integration show <name>
weaver integration list
weaver integration remove <name>
weaver integration export <name> [--json]
weaver integration import <file>
weaver group create <name> <branch...>
weaver group add <name> <branch...>
weaver group remove <name> [branch...]
weaver group list
weaver export
weaver import <file>
```

## Quick Start

Install the latest tagged release:

```bash
go install github.com/lutefd/weaver@latest
weaver version
```

Initialize Weaver inside a Git repository:

```bash
weaver init
```

Declare a stack:

```bash
weaver stack feature-b --on feature-a
weaver stack feature-c --on feature-b
```

Inspect it:

```bash
weaver deps feature-c
weaver status
weaver doctor
```

Refresh local branches from upstream:

```bash
weaver update main feature-a feature-b
weaver update --integration integration
weaver update --all
```

Rebase it:

```bash
weaver sync feature-c
```

Compose it:

```bash
weaver compose feature-c --dry-run
weaver compose feature-c
weaver compose feature-c --base main --create integration
weaver compose feature-c --base main --update integration
weaver integration save integration --base main feature-a feature-b feature-c
weaver compose --integration integration --update integration
```

## Files and State

Weaver uses local metadata inside `.git/weaver/`.

- `.git/weaver/deps.yaml`: branch dependency declarations.
- `.git/weaver/groups.yaml`: named compose groups.
- `.git/weaver/integrations.yaml`: saved integration compose strategies.
- `.git/weaver/rebase-state.yaml`: in-progress rebase resume state.
- `.weaver.yaml`: repository-level config.

None of the `.git/weaver/` files are intended to be committed.

## Safety Model

- Rebase operations use `git rebase --autostash`.
- `weaver update` runs `git fetch --all` and fast-forwards selected local branches to their upstream refs.
- Mutating Git commands are printed before execution.
- Rebase state is persisted before each step.
- `weaver abort` restores the original branch.
- `weaver compose` is ephemeral by default.
- `weaver compose --integration <name>` reuses the saved base and branch set from that integration strategy.
- If a very divergent branch keeps breaking a large compose, remove it from that compose or integration first, fix or merge it manually, then add it back once it is stable again.
- `weaver compose --create <branch>` creates a new integration branch from the composed result.
- `weaver compose --update <branch>` rebuilds an existing integration branch from the clean base and force-moves it to the new composed result.

## Docs

- [Architecture](./docs/architecture.md)
- [Usage Guide (English)](./docs/usage.en.md)
- [Guia de Uso (Português do Brasil)](./docs/usage.pt-br.md)
- [Guía de Uso (Español Argentina)](./docs/usage.es-ar.md)

## Verification

Run unit tests:

```bash
go test ./...
```

Run integration tests against real temporary repositories:

```bash
make test-integration
```

Run the end-to-end smoke test:

```bash
./smoketest.sh
```

The smoke script writes a step-by-step log to `smoketest.log`, uses temporary repositories, and removes them when it exits.
