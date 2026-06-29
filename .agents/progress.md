# Progress

## 2026-06-29

- Created initial Go workload generator structure.
- Added heterogeneous virtual users and workload profiles.
- Added AdventureWorks SQL operations for catalog, sales, inventory, purchasing, HR, and optional cart writes.
- Added final Markdown/JSON reporting.
- Added README and agent handoff notes.

## Next Useful Work

- Run against a real AdventureWorks2022 instance and tune query weights.
- Add optional Prometheus/OpenTelemetry export if cluster tests need live dashboards.
- Add a cleanup-only command for old `awload-%` cart rows.
- Consider CSV comparison output for repeated benchmark runs.

