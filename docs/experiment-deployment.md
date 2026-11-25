# Experiment deployment

Use a separate unified configuration and runtime directory for each topology. The runtime contains certificates, ledger data, publication state, process records, logs, and verification reports. Tools and compiled commands are shared from the checkout.

## Select a topology

Copy the repository-root config.yaml to config/experiment.yaml. Set runtime to ../runtime/experiments/example, then edit network and services. Relative paths resolve against the configuration file's directory.

~~~bash
./run.sh plan --config config/experiment.yaml
./run.sh up --config config/experiment.yaml
./run.sh status --config config/experiment.yaml
./run.sh down --config config/experiment.yaml
~~~

Plan prints endpoints and policies before starting processes. Use a fresh runtime and available ports for a new topology; existing channel membership persists across restarts.

## Organization and port settings

The network section sets organizations, peers_per_organization, orderers, endorsement_threshold, gateway_concurrency, and port_base. The default business endorsement threshold is a strict majority of organizations. Each organization contributes one endorsement regardless of its peer count. Chaincode lifecycle approval also uses a majority; Raft membership is set by the orderer count.

Gateway concurrency limits concurrent RPCs per peer. Record it with benchmark settings. Set service ports independently under services.

Orderer i uses port_base + 10*i for its endpoint and the next two ports for operations and administration. Peer k uses port_base + 1000 + 10*k and the next three ports for its chaincode listener, external chaincode server, and operations endpoint. Indices start at zero; peers are numbered in organization order. Plan prints the concrete allocation, and startup checks ports.

## Multiple machines

The controller requires the installed project, Linux amd64, Python, PyYAML, SSH, and rsync. Workers require Linux amd64, Python 3, rsync, and a writable dedicated runtime directory. Configure noninteractive SSH access and reachable advertised peer and orderer endpoints.

The [README example](../README.md#run-across-multiple-machines) provides a complete two-host configuration. Within network, a host without ssh is local; a host with ssh is remote. Optional ssh_port and identity_file fields select its connection. The root field sets its dedicated runtime directory.

Placement lists assign peers and orderers to named hosts. Without explicit placement, nodes are distributed round-robin. The controller transfers binaries, certificates, and configurations, then manages local and remote processes. Each peer has a colocated chaincode server. The API, publisher, Redis, and CoreDNS run on the controller with loopback listeners.

Certificates and channel configuration use the advertised addresses. Peer and orderer endpoints must be reachable among nodes and from the controller. Chaincode, operations, and administration endpoints remain on each node's loopback interface.

## Verification and measurement

For a configuration using runtime/experiments/example:

~~~bash
export DIS_RUNTIME="$PWD/runtime/experiments/example"
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
~~~

These checks exercise the signed workflow, peers, Gateway discovery, replica endorsements, DNS publication, retries, and recovery. Use a dedicated test deployment. Save topology, effective configuration, source revision, and verification reports with measurements. See [verification](verification.md) and the [Caliper workload](../benchmarks/caliper/README.md).

## Direct network manager

The deploy/topologies directory contains network-only examples for scripts/network.py. For component-level use, set DIS_RUNTIME and pass --topology PATH when creating the network; start scripts/services.py with the desired service ports. The unified launcher combines these settings in one configuration file.
