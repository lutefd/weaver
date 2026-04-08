# Architecture

## Design Goals

Weaver is built around a few constraints:

- Local-first: stack declarations live in `.git/weaver/`.
- Git-native: Git remains the execution engine.
- Offline-friendly: the workflow must not depend on GitHub.
- Crash-safe: multi-step rebase operations persist progress.
- Safe integration by default: compose should stay ephemeral unless the user explicitly requests `--create` or `--update`.
- Remote refresh: upstream-tracking updates should be explicit and fast-forward only.

## Layout

```text
weaver/
├── cmd/                  # Cobra commands
├── internal/config/      # Repo-level config (.weaver.yaml)
├── internal/git/         # Git runner and parsers
├── internal/deps/        # Local dependency storage
├── internal/doctor/      # Repository diagnostics and validation
├── internal/stack/       # DAG model and health checks
├── internal/updater/     # Remote-aware branch fast-forward engine
├── internal/resolver/    # DAG construction from dependency sources
├── internal/rebaser/     # Ordered stack rebase + persisted resume state
├── internal/composer/    # Ephemeral multi-branch compose engine
├── internal/integration/ # Saved integration strategy storage + sharing
├── internal/group/       # Named compose groups
├── internal/portability/ # Export / import support
└── internal/ui/          # Tree and chain rendering
```

## Command Layer

The `cmd/` package is intentionally thin. Each Cobra command owns:

- argument and flag validation
- user-facing output
- orchestration of internal packages

The heavy behavior lives in `internal/`, which is where most of the logic and tests sit.

## Dependency Model

Dependencies are stored as child-to-parent edges:

```yaml
version: 1
dependencies:
  feature-b: feature-a
  feature-c: feature-b
```

The resolver loads those edges and builds a DAG. The stack package provides:

- cycle detection
- ancestor resolution
- topological ordering
- tree rendering inputs

## Health Model

`weaver status` evaluates each branch against its parent, or against the configured base for root branches.

Current health states:

- `clean`
- `needs rebase`
- `conflict risk`

The check uses `git merge-base`, `git rev-parse`, and `git merge-tree --write-tree`.

## Doctor

`weaver doctor` is a read-only diagnostic pass over repository and Weaver state.

It checks:

- Weaver initialization files
- config loading and base branch existence
- dependency graph validity and referenced branches
- group file validity and referenced branches
- integration strategy validity and referenced branches
- pending rebase state sanity
- detached HEAD, dirty working tree, and in-progress Git operations

## Rebase Engine

`weaver sync` resolves the ancestor chain for the target branch and rebases from the base upward.

State is written to `.git/weaver/rebase-state.yaml` before each step so `weaver continue` and `weaver abort` can recover safely.

Key properties:

- every rebase uses `--autostash`
- the original branch is restored after success or abort
- mutating Git commands are printed before execution

## Merge Sync Engine

`weaver sync --merge` resolves the same ancestor chain but merges each parent into its child in order, allowing fast-forwards when Git can do them cleanly.

State is written to `.git/weaver/merge-state.yaml` before each step so `weaver continue` and `weaver abort` can recover safely without rewriting branch history.

## Update Engine

`weaver update` accepts:

- explicit branches
- a named group via `--group`
- a saved integration strategy via `--integration`
- every tracked branch via `--all`

The engine:

1. Resolves the selected local branches.
2. Runs `git fetch --all` once.
3. Resolves each branch's configured upstream ref.
4. Checks out each branch in turn and fast-forwards it with `git merge --ff-only <upstream>`.
5. Restores the original branch.

If a branch has no upstream, does not exist locally, or cannot be fast-forwarded, the update stops and the original branch is restored.

## Compose Engine

`weaver compose` accepts:

- explicit branches
- a named group via `--group`
- a saved integration strategy via `--integration`
- every tracked branch via `--all`
- an optional `--base <branch>` override for the composition target

Saved integration strategies live in `.git/weaver/integrations.yaml` and record:

- a reusable integration name
- the base branch
- the explicit branch list to compose in a shared, reproducible way

The engine:

1. Expands each requested branch to its ancestor chain.
2. Unions and deduplicates those branches.
3. Produces a stable parent-before-child order.
4. Checks out detached `HEAD` at the base branch.
5. Merges each branch in order.
6. Optionally creates a new branch from the composed result when `--create` is requested.
7. Optionally force-updates another branch from the clean composed result when `--update` is requested.
8. Restores the original branch.

If a merge fails, the compose operation aborts and restores the prior branch.

By default compose is ephemeral. New integration branches can be created explicitly with `--create`, and existing integration branches can be rebuilt from scratch with `--update`.

## Portability

Weaver can export local orchestration state to JSON:

- dependencies
- groups
- integrations
- export timestamp

That file can be imported into another clone by a different orchestrator without needing to reconstruct the stack manually.
