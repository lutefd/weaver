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
```

## Rebase

```bash
weaver update main feature-a feature-b
weaver update --integration integration
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
If one branch keeps breaking a large compose because it has drifted too far, remove it from that compose or integration, repair it, and then merge it manually onto the branch produced by `--create` or `--update` before adding it back.

## Integrations

```bash
weaver integration save integration --base main feature-a feature-b feature-c
weaver integration show integration
weaver integration doctor integration
weaver integration doctor integration --json
weaver integration list
weaver integration remove integration
weaver integration export integration --json > integration.json
weaver integration import integration.json
```

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
