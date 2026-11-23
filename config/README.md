# Unified deployment configuration

The repository-root `config.yaml` configures the Fabric topology, runtime directory, and API, DNS, and Redis ports. The unified launcher uses the existing network and service managers.

## Start and inspect

Install dependencies and build the commands using the [running guide](../docs/runbook.md), then run from the repository root:

~~~bash
python3 scripts/dis.py plan
python3 scripts/dis.py up
python3 scripts/dis.py status
python3 scripts/dis.py down
~~~

The supplied configuration creates four organizations, one peer per organization, four Raft orderers, and a three-organization endorsement policy. API, DNS, and Redis listen on loopback ports 38080, 38530, and 38679. Fabric ports start at 37000; runtime data is stored under runtime/configured.

The plan command validates and prints effective settings without creating runtime files or starting processes. The up command starts Fabric first, then Redis, the event publisher, CoreDNS, and the API. The down command stops services before Fabric and retains data. If service startup fails after Fabric starts, inspect logs and retry up or run down.

## Configuration fields

| Field | Meaning |
|---|---|
| version | Configuration schema version; currently 1 |
| runtime | Dedicated deployment data directory; relative paths resolve against the configuration file's directory |
| network.organizations | Number of peer organizations |
| network.peers_per_organization | Peer replicas in each organization |
| network.orderers | Raft ordering nodes |
| network.endorsement_threshold | Number of distinct organizations required to endorse a business transaction |
| network.gateway_concurrency | Concurrent Gateway RPC limit per peer; this is not a TPS limit |
| network.port_base | Base for generated Fabric ports |
| network.domain | Domain bound into signed business requests |
| network.hosts | Machine addresses and optional SSH worker settings |
| network.placement | Optional peer and orderer placement lists |
| services.api_port | HTTP API port |
| services.dns_port | Authoritative DNS UDP/TCP port |
| services.redis_port | Redis port |

The [deployment guide](../docs/experiment-deployment.md) describes topology fields, port allocation, and multi-machine placement. API, Redis, CoreDNS, and the publisher run on the controller. The current service manager binds API, DNS, and Redis to loopback. Channel and chaincode names remain dis; tool versions are pinned by bootstrap.py. Signing rules, DNS TTL, and application timeouts remain code settings.

## Select another configuration

Copy config.yaml and change settings, then use the same file for every command:

~~~bash
python3 scripts/dis.py plan --config config/experiment.yaml
python3 scripts/dis.py up --config config/experiment.yaml
python3 scripts/dis.py status --config config/experiment.yaml
python3 scripts/dis.py down --config config/experiment.yaml
~~~

A copied file's relative runtime path resolves against its new directory. Use an absolute runtime path or adjust the relative path when moving configuration files. Keep local experiment configuration, generated credentials, and measurements outside version control.

The unified launcher's runtime value overrides the DIS_RUNTIME environment variable for child commands. Existing network.py, services.py, topology files, and their command-line options remain available. When calling those scripts directly, set DIS_RUNTIME to the same absolute directory and use matching ports.

A runtime retains its topology. Use a new runtime directory and available ports for a different organization count, endorsement policy, node placement, or Gateway limit. Editing YAML does not reconfigure existing channel membership. To change service ports in the same runtime, stop the deployment first, edit the ports, and run up again.

The default unified configuration is a separate deployment from the legacy default runtime. Check port availability and select the intended configuration before starting a second deployment.
