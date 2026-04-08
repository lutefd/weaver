# Weaver Command Reference

Use `weaver` as the executable name.

## Setup

```bash
weaver init
weaver version
```

## Dependencies

```bash
weaver stack feature-b --on feature-a
weaver stack feature-c --on feature-b
weaver unstack feature-c
weaver deps
weaver deps feature-c
weaver status
```

## Rebase

```bash
weaver update main feature-a feature-b
weaver update --group sprint-42
weaver update --all
weaver sync
weaver sync feature-c
weaver continue
weaver abort
```

## Compose

```bash
weaver compose feature-c --dry-run
weaver compose feature-c --base main --create integration --dry-run
weaver compose feature-c --base main --replace integration --dry-run
weaver compose feature-a feature-c feature-e
weaver compose --group sprint-42
weaver compose --group sprint-42 --dry-run
weaver compose --all
weaver compose feature-b feature-d --base main --create integration
weaver compose feature-b feature-d --base main --replace integration
```

Selection rule: use exactly one of explicit branches, `--group`, or `--all`.

`weaver compose ... --persist` still exists for the narrow case where you intentionally want to move the base branch itself, but it is deprecated for integration-branch workflows.

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
