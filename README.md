# Weaver

Weaver is a local-first Go CLI for managing stacked Git branches without requiring GitHub metadata, PR conventions, or external services.

It stores stack relationships in `.git/weaver/`, keeps them out of the committed tree, and uses regular Git commands under the hood so the workflow stays inspectable.

## What It Does

- Declares branch dependencies locally with `weaver stack`.
- Resolves stacks as a DAG built from `.git/weaver/deps.yaml`.
- Shows stack structure and health with `weaver deps` and `weaver status`.
- Rebases a stack in dependency order with crash-safe resume state.
- Composes multiple branches into an ephemeral integration state.
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
weaver sync [branch]
weaver continue
weaver abort
weaver compose [branch...] [--group NAME | --all] [--dry-run]
weaver group create <name> <branch...>
weaver group add <name> <branch...>
weaver group remove <name> [branch...]
weaver group list
weaver export
weaver import <file>
```

## Quick Start

Build the CLI:

```bash
make build
```

Initialize Weaver inside a Git repository:

```bash
./bin/weaver init
```

Declare a stack:

```bash
./bin/weaver stack feature-b --on feature-a
./bin/weaver stack feature-c --on feature-b
```

Inspect it:

```bash
./bin/weaver deps feature-c
./bin/weaver status
```

Rebase it:

```bash
./bin/weaver sync feature-c
```

Compose it:

```bash
./bin/weaver compose feature-c --dry-run
./bin/weaver compose feature-c
```

## Files and State

Weaver uses local metadata inside `.git/weaver/`.

- `.git/weaver/deps.yaml`: branch dependency declarations.
- `.git/weaver/groups.yaml`: named compose groups.
- `.git/weaver/rebase-state.yaml`: in-progress rebase resume state.
- `.weaver.yaml`: repository-level config.

None of the `.git/weaver/` files are intended to be committed.

## Safety Model

- Rebase operations use `git rebase --autostash`.
- Mutating Git commands are printed before execution.
- Rebase state is persisted before each step.
- `weaver abort` restores the original branch.
- `weaver compose` uses detached `HEAD` and does not create a saved integration branch.

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

Run the end-to-end smoke test:

```bash
./smoketest.sh
```

The smoke script writes a step-by-step log to `smoketest.log`, uses temporary repositories, and removes them when it exits.
