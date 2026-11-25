# Operational scripts

Use ./run.sh with [config.yaml](../config.yaml) for routine startup, status, and shutdown. The underlying scripts support component management, verification, and experiments.

| Script | Purpose |
|---|---|
| dis.py | Unified configuration and deployment orchestration |
| configuration.py | Validate unified settings and resolve the runtime directory |
| bootstrap.py | Install pinned Fabric, CoreDNS, and Redis tools |
| network.py | Plan and manage Fabric networks on one or multiple machines |
| services.py | Manage Redis, publisher, CoreDNS, and API |
| topology.py | Validate organizations, nodes, endorsement, ports, and placement |
| transport.py / node-agent.py | Deploy and manage remote node processes over SSH |
| acceptance.py | Verify business workflow, peers, Gateways, DNS, and restart |
| acceptance-events.py | Verify publisher recovery after offline commits |
| benchmark-caliper.py | Run the signed-registration Caliper workload |
| benchmark-paper.py | Measure registration, ledger reads, or DNS queries |
| benchmark-scale.py | Measure registration across organizational scales |
| smoke.py | Exercise the development backend and request signing |

## Select a deployment for direct commands

The launcher sets DIS_RUNTIME from the configuration. When running scripts directly, set it explicitly:

~~~bash
export DIS_RUNTIME="$PWD/runtime/configured"
python3 scripts/network.py status
python3 scripts/services.py status
~~~

Acceptance and benchmark scripts use this runtime to find the selected network. Reports are stored under the chosen runtime or benchmark output directory. See [verification](../docs/verification.md), [deployment](../docs/experiment-deployment.md), and [Caliper](../benchmarks/caliper/README.md).
