# Weaver Command Reference

Use `weaver` as the executable name.

## Setup

```bash
weaver init
weaver version
weaver doctor
weaver doctor --json
```

## Dependencies

```bash
weaver stack feature-b --on feature-a
weaver stack feature-c --on feature-b
weaver unstack feature-c
weaver deps
weaver deps feature-c
weaver status
weaver status --upstream
```

`weaver status` compares each branch with its declared stack parent or base.
`weaver status --upstream` fetches remote refs and compares each tracked branch with its configured upstream without fast-forwarding local branches.

## Rebase

```bash
weaver update main feature-a feature-b
weaver update --integration integration
weaver update --group sprint-42
weaver update --all
weaver sync
weaver sync feature-c
weaver sync feature-c --merge
weaver continue
weaver abort
```

Use `weaver sync feature-c --merge` when you want to preserve existing branch history, such as open PRs. It fast-forwards when possible and otherwise creates the normal merge commit.

## Compose

```bash
weaver compose feature-c --dry-run
weaver compose --integration integration --dry-run
weaver compose --integration integration --create integration-preview --skip feature-debug-search-api-curl
weaver compose feature-c --base main --create integration --dry-run
weaver compose feature-c --base main --update integration --dry-run
weaver compose feature-a feature-c feature-e
weaver compose --group sprint-42
weaver compose --group sprint-42 --dry-run
weaver compose --all
weaver compose feature-b feature-d --base main --create integration
weaver compose feature-b feature-d --base main --update integration
```

Selection rule: use exactly one of explicit branches, `--group`, `--integration`, or `--all`.
If no `--skip` is provided and a branch conflicts, Weaver prompts for `skip` or `abort`.
If one branch keeps breaking a large compose, prefer `--skip` first: let the compose finish, then merge that branch manually onto the branch produced by `--create` or `--update`.
If the same branch is repeatedly the outlier across runs, it can be better to pass `--skip <branch>` up front instead of waiting for the conflict prompt.
Only remove the branch from the compose or saved integration entirely when skip-and-manual-merge is no longer a practical short-term workflow.
Weaver tracks branches targeted by both `--create` and `--update`, so `weaver integration branch list` and `weaver integration branch delete <name>` work for either path.
`weaver integration branch list` reports which branches were composed originally, which skipped branches were merged later, and which skipped branches are still pending.

## Integrations

```bash
weaver integration save integration --base main feature-a feature-b feature-c
weaver integration show integration
weaver integration doctor integration
weaver integration doctor integration --json
weaver integration list
weaver integration remove integration
weaver integration branch list
weaver integration branch delete integration-preview
weaver integration export integration --json > integration.json
weaver integration import integration.json
```

On an interactive terminal, `weaver integration branch list` opens a Bubble Tea browser. Use `up`/`down` to move, `d` to delete, `r` to refresh, and `q` to quit.

## Groups

```bash
weaver group create sprint-42 feature-a feature-b feature-c
weaver group add sprint-42 feature-d
weaver group remove sprint-42 feature-c
weaver group remove sprint-42
weaver group list
```

## Portability

```bash
weaver export > weaver-state.json
weaver import weaver-state.json
```

Portable state includes dependencies, groups, saved integrations, and tracked integration branches.
