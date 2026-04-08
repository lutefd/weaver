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
weaver sync
weaver sync feature-c
weaver continue
weaver abort
```

## Compose

```bash
weaver compose feature-c --dry-run
weaver compose feature-c --base integration --persist --dry-run
weaver compose feature-a feature-c feature-e
weaver compose --group sprint-42
weaver compose --group sprint-42 --dry-run
weaver compose --all
weaver compose feature-b feature-d --base integration --persist
```

Selection rule: use exactly one of explicit branches, `--group`, or `--all`.

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
