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
weaver sync [branch] [--merge]
weaver continue
weaver abort
weaver compose [branch...] [--group NAME | --integration NAME | --all] [--base BRANCH] [--create BRANCH | --update BRANCH] [--skip BRANCH...] [--dry-run]
weaver integration save <name> <branch...> [--base BRANCH]
weaver integration show <name>
weaver integration doctor <name> [--json]
weaver integration list
weaver integration remove <name>
weaver integration branch list
weaver integration branch delete <name>
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

Sync it with rebase:

```bash
weaver sync feature-c
```

Sync it with merge commits instead:

```bash
weaver sync feature-c --merge
```

Compose it:

```bash
weaver compose feature-c --dry-run
weaver compose --integration integration --dry-run
weaver compose feature-c
weaver compose feature-c --base main --create integration
weaver compose feature-c --base main --update integration
weaver integration save integration --base main feature-a feature-b feature-c
weaver integration doctor integration
weaver compose --integration integration --create integration-preview --skip feature-c
weaver compose --integration integration --update integration
weaver integration branch list
weaver integration branch delete integration-preview
```

## Choosing a Command

- Use `weaver stack` and `weaver unstack` to declare or change parent-child relationships between branches.
- Use `weaver deps`, `weaver status`, and `weaver doctor` to inspect the stack before changing anything.
- Use `weaver update` to refresh local branches from their own configured upstream refs. This does not sync a branch with its stack parent or with the repo base branch.
- Use `weaver sync` to bring a stack back into dependency order by applying each parent into its child.
- Use `weaver compose` to test or materialize the combined result of several branches on top of a base branch without modifying the source branches themselves.
- Use `weaver integration ...` when the same compose recipe should be reusable and shared by name.
- Use `weaver integration branch ...` when you want to inspect or clean up materialized branches previously created or refreshed with `weaver compose --create` or `weaver compose --update`.
  On an interactive terminal, `weaver integration branch list` opens a Bubble Tea browser with keyboard shortcuts for navigation, refresh, and delete.

## Choosing Rebase or Merge

- Use `weaver sync` when you want the clean stacked-diff workflow: linear branch history, parent changes replayed into children, and you are comfortable force-pushing updated branches afterward.
- Use `weaver sync --merge` when the branches already have open PRs, review comments, or other consumers and you want to preserve existing branch history instead of rewriting it.
- Rebase produces cleaner history, but everyone touching that stack needs to be disciplined about rebasing and force-pushing.
- Merge preserves branch history, but it accumulates merge commits and usually gets noisier over time.
- `weaver update` is not a substitute for either strategy. It only fast-forwards each branch from its configured upstream.

## Files and State

Weaver uses local metadata inside `.git/weaver/`.

- `.git/weaver/deps.yaml`: branch dependency declarations.
- `.git/weaver/groups.yaml`: named compose groups.
- `.git/weaver/integrations.yaml`: saved integration compose strategies.
- `.git/weaver/rebase-state.yaml`: in-progress rebase resume state.
- `.git/weaver/merge-state.yaml`: in-progress merge-sync resume state.
- `.weaver.yaml`: repository-level config.

None of the `.git/weaver/` files are intended to be committed.

## Safety Model

- Rebase operations use `git rebase --autostash`.
- Merge-based stack sync uses `git merge --no-edit`, fast-forwards when possible, and otherwise creates the normal autogenerated merge commit while keeping history stable for already-open PRs.
- `weaver update` runs `git fetch --all` and fast-forwards selected local branches to their upstream refs.
- Mutating Git commands are printed before execution.
- Stack sync state is persisted before each step.
- `weaver abort` restores the original branch.
- `weaver compose` is ephemeral by default.
- `weaver compose --integration <name>` reuses the saved base and branch set from that integration strategy.
- `weaver integration doctor <name>` checks whether a saved integration is coherent, including drift, foreign ancestry, and suspicious merge history.
- Compose failures report the branch that failed and the conflicting files.
- `weaver compose --skip <branch>` leaves that branch out of the resolved compose order so you can merge it manually later.
- If you do not pass `--skip`, compose will prompt you to skip the failing branch or abort.
- If a very divergent branch keeps breaking a large compose, remove it from that compose or integration first, repair it, and then merge it manually onto the branch produced by `weaver compose --create <branch>` or `--update <branch>` before adding it back once it is stable again.
- `weaver compose --create <branch>` creates a new integration branch from the composed result.
- Weaver tracks branches targeted by both `weaver compose --create <branch>` and `weaver compose --update <branch>` locally so you can list and delete them later with `weaver integration branch list` and `weaver integration branch delete <branch>`.
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
