# Deployment configuration

The repository-root [config.yaml](../config.yaml) selects the runtime, Fabric topology, and service ports. After [installation](../docs/runbook.md), run from the repository root:

~~~bash
./run.sh plan
./run.sh
./run.sh status
./run.sh down
~~~

With no arguments, run.sh starts the deployment. Plan validates and prints settings. Startup launches Fabric, then Redis, the publisher, CoreDNS, and the API, and reports readiness. Shutdown stops services before Fabric and retains data.

## Fields

| Field | Meaning |
|---|---|
| version | Configuration schema version: 1 |
| runtime | Deployment data directory; relative to the configuration file |
| network.organizations | Peer organization count |
| network.peers_per_organization | Peers in each organization |
| network.orderers | Raft orderer count |
| network.endorsement_threshold | Distinct organizations required for business endorsement |
| network.gateway_concurrency | Concurrent Gateway RPC limit per peer |
| network.port_base | Base for generated Fabric ports |
| network.domain | Domain bound into signed requests |
| network.hosts | Advertised machine addresses and SSH worker settings |
| network.placement | Peer and orderer assignments to hosts |
| services.api_port | HTTP API port |
| services.dns_port | Authoritative DNS UDP/TCP port |
| services.redis_port | Redis port |

Defaults are four organizations, one peer each, four orderers, endorsement threshold three, and Gateway concurrency 500. Fabric ports start at 37000. API, DNS, and Redis use loopback ports 38080, 38530, and 38679. Data is stored under runtime/configured. Channel and chaincode names are dis. Bootstrap pins tool versions; request encoding, DNS TTL, and application timeouts are code settings.

## Another deployment

Copy config.yaml to config/experiment.yaml, set runtime to ../runtime/experiments/example, and edit the topology and ports. Use the same file for each command:

~~~bash
./run.sh plan --config config/experiment.yaml
./run.sh up --config config/experiment.yaml
./run.sh status --config config/experiment.yaml
./run.sh down --config config/experiment.yaml
~~~

Use a new runtime and available ports for a different topology. Existing runtime data retains channel membership. For service-port changes, stop the deployment, edit the configuration, then start it again. Keep local experiment settings and generated results under runtime.

See [deployment](../docs/experiment-deployment.md) for machine requirements, SSH settings, placement, and port allocation. API, Redis, CoreDNS, and the publisher run on the controller.

## Launcher and logs

run.sh selects .venv/bin/python when available and otherwise python3. Set PYTHON to override the interpreter. A relative --config path is resolved from the caller's working directory. The configured runtime is passed to child processes as DIS_RUNTIME.

Startup prints the configuration path, runtime, domain, node counts, endorsement policy, Gateway concurrency, node endpoints, service listeners, and log directories. “DIS is ready” follows successful readiness checks. Managed processes continue in the background. Local logs are under the runtime's fabric/ and services/ directories; remote node logs are under the worker runtime.

For direct component commands, set DIS_RUNTIME explicitly and supply matching ports. Without this variable, the direct managers use runtime/ and service ports 28080, 28530, and 28679.
