# Operational scripts

Run scripts from the repository root. The [running guide](../docs/runbook.md) gives installation and interactive commands.

| Script | Purpose |
|---|---|
| bootstrap.py | Install pinned Fabric, CoreDNS, and Redis tools |
| network.py | Create and manage the scoped Fabric network |
| services.py | Manage Redis, the event publisher, CoreDNS, and the API |
| upgrade-ledger.py | Upgrade an initialized v1 network and migrate state in place |
| acceptance.py --restart | Verify signed workflow, four-peer queries, event publication, DNS AXFR, replay, and restart |
| acceptance-events.py | Stop the publisher, submit offline updates, and verify checkpoint catch-up |
| smoke.py | Verify the development file backend and CLI signing |

Acceptance reports are written beneath runtime. Managed services remain running after connected acceptance. The event acceptance requires exclusive control of the project's publisher process.
