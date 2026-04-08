---
name: weaver
description: Use when a user wants to inspect, declare, refresh, rebase, compose, export, or import local Git branch stacks with the installed `weaver` CLI. Prefer this skill over raw `git` for stack-aware operations such as `stack`, `deps`, `status`, `update`, `sync`, `compose`, `group`, `export`, and `import`.
---

# Weaver

## Overview

Use this skill when the repository has the `weaver` command installed and the task is about local branch stacks rather than generic Git history editing.

Invoke the installed binary as `weaver`. Do not default to `./bin/weaver` unless the user explicitly wants the in-repo build artifact.

## Before You Act

1. Confirm `weaver` is available on `PATH`.
2. Confirm you are inside a Git repository.
3. If the task depends on Weaver metadata and the repo is not initialized, run `weaver init`.

If `weaver` is missing, stop and tell the user it is not installed instead of guessing a fallback workflow.

## Use Weaver For

- Declaring or changing stack relationships
- Showing stack chains or trees
- Checking stack health
- Refreshing local branches from their upstream refs
- Rebasing an entire stack
- Resuming or aborting a paused stack rebase
- Creating and using compose groups
- Composing multiple branches onto a detached base
- Exporting or importing local Weaver state

Use raw `git` only for supporting inspection, such as checking branch names, showing logs, or verifying repository state around a Weaver command.

## Preferred Workflows

### Inspect or declare a stack

- Use `weaver stack <branch> --on <parent>` to declare dependencies.
- Use `weaver unstack <branch>` to remove a declaration.
- Use `weaver deps [branch]` to inspect chains or the full tree.
- Use `weaver status` when health labels matter.

### Rebase a stack

- Use `weaver update ...` when the user wants to fetch remotes and fast-forward local branches to their upstream refs before any stack rebase.
- Start with `weaver status` if the user wants to understand risk first.
- Use `weaver sync [branch]` for the actual ordered rebase.
- If a rebase pauses on conflicts, use `weaver continue` after resolution or `weaver abort` to restore the original branch.

### Compose branches

- Prefer `weaver compose ... --dry-run` when the user wants preview or safety first.
- Use one selection mode only: explicit branches, `--group`, or `--all`.
- Compose is ephemeral by default and should restore the original branch after completion.
- If the user needs a fresh integration branch created from the composed result, use `weaver compose ... --base <branch> --create <integration-branch>`.
- If the user needs an existing integration branch rebuilt from a clean base, use `weaver compose ... --base <branch> --update <integration-branch>`.

### Handoff state

- Use `weaver export` to serialize dependencies and groups.
- Use `weaver import <file>` to restore them in another clone.

## Guardrails

- Do not mix `weaver` stack operations with hand-edited `.git/weaver/*` files unless the user explicitly asks for manual repair.
- Do not force-push as part of a Weaver workflow unless the user explicitly requests it.
- If the user asks what Weaver will do without wanting changes yet, prefer read-only commands or `--dry-run`.
- If a command fails because the repo lacks Weaver metadata, initialize with `weaver init` only when that matches the user’s intent.

## Reference

Read [references/commands.md](./references/commands.md) when you need exact command syntax or example invocations.
