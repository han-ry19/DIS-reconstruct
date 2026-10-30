# Operational scripts

Run scripts from the repository root. The [running guide](../docs/runbook.md) gives installation and interactive commands.

| Script | Purpose |
|---|---|
| bootstrap.py | Install pinned Fabric, CoreDNS, and Redis tools |
| network.py | Plan and manage configurable Fabric networks on one or more machines |
| services.py | Manage Redis, the event publisher, CoreDNS, and the API |
| upgrade-ledger.py | Upgrade an initialized v1 network and migrate state in place |
| acceptance.py --restart --all-gateways | Verify signed workflow, every peer and Gateway, replica endorsement, event publication, DNS AXFR, replay, and restart |
| acceptance-events.py | Stop the publisher, submit offline updates, and verify checkpoint catch-up |
| benchmark-paper.py | Measure registration, committed-state reads, or DNS under a fixed offered-load sweep |
| benchmark-scale.py | Measure majority-endorsed registration across configurable organization counts |
| summarize-benchmarks.py | Generate aggregate tables, figures, and a reproducible benchmark report |
| smoke.py | Verify the development file backend and CLI signing |

Use DIS_RUNTIME to isolate deployment state and acceptance reports for each experiment. The [experiment deployment guide](../docs/experiment-deployment.md) describes topology files and SSH workers.

Acceptance reports are written beneath DIS_RUNTIME. Managed services remain running after connected acceptance. The event acceptance requires exclusive control of the project's publisher process.
