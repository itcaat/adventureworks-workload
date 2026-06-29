# Agent Notes

## Project Goal

Build a Go-based AdventureWorks workload generator for Microsoft SQL Server cluster testing. The tool should mimic application traffic rather than validate AdventureWorks business logic.

## Current State

- CLI entrypoint: `cmd/awload/main.go`.
- Main package logic: `internal/app`.
- Supported profiles: `mixed`, `read-heavy`, `reporting`, `write-light`.
- Write workload is deliberately gated behind `-write-mode cart`.
- Final reports are Markdown and JSON under `reports/`.

## Design Rules

- Prefer realistic, parameterized SQL over schema-edge-case tests.
- Keep read-only as the safe default.
- Any write workload must be scoped, recognizable, and easy to clean up.
- Do not hard-code environment-specific server names or passwords.
- Preserve final report compatibility when adding metrics.

## Verification

Run:

```bash
go test ./...
go build ./cmd/awload
```

Database integration requires a reachable SQL Server with AdventureWorks2022 restored.

