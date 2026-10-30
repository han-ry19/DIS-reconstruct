# Deploying experiments at different scales

A topology file sets the number of organizations, peers per organization, Raft orderers, organization endorsement threshold, and node placement. Each experiment uses a separate DIS_RUNTIME directory for its certificates, ledgers, publication state, process records, logs, and acceptance reports. Fabric tools and compiled commands are shared from the checkout.

## Single-machine deployment

The supplied configurations include three organizations with two peers each and five organizations with one peer each. Both use three orderers. Create the first experiment with:

~~~bash
export DIS_RUNTIME="$PWD/runtime/experiments/local-3org"
python3 scripts/network.py plan --topology deploy/topologies/local-3org.yaml
python3 scripts/network.py up --topology deploy/topologies/local-3org.yaml
python3 scripts/services.py up --api-port 45080 --dns-port 45530 --redis-port 45679
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
~~~

The plan command prints the effective topology, allocated ports, and endorsement policy without starting processes. Change DIS_RUNTIME when selecting another topology. On subsequent starts, network.py reads the saved topology; --topology is optional and must match the saved configuration when supplied.

## Topology fields

~~~yaml
organizations: 3
peers_per_organization: 2
orderers: 3
endorsement_threshold: 2
port_base: 17000
hosts:
  local:
    address: 127.0.0.1
~~~

The default organization endorsement threshold is a strict majority. An organization contributes one endorsement to the threshold regardless of its number of peers. Raft ordering membership is configured separately. The chaincode lifecycle approval policy uses a majority of organizations.

Orderer i uses port_base + 10*i for its Fabric endpoint and the next two ports for operations and administration. Peer k uses port_base + 1000 + 10*k and the next three ports for its peer chaincode listener, external chaincode server, and operations endpoint. Peer k is numbered in organization order. The manager checks the selected ports before creating a network. Choose API, DNS, and Redis ports separately with services.py; use nonoverlapping ranges for concurrently running experiments.

## Multiple machines

Use a Linux amd64 controller with the built checkout, Python, PyYAML, SSH, and rsync. Each worker needs Linux amd64, Python 3, and rsync. Configure noninteractive SSH access from the controller to the worker account and verify the worker host key before deployment. The account must own its dedicated runtime directory.

Copy deploy/topologies/two-host.yaml and fill in reachable addresses, SSH targets, and worker runtime paths:

~~~yaml
organizations: 3
peers_per_organization: 2
orderers: 3
endorsement_threshold: 2
port_base: 21000
hosts:
  controller:
    address: controller.example.org
  worker:
    address: worker.example.org
    ssh: experiment@worker.example.org
    ssh_port: 22
    identity_file: /home/experiment/.ssh/experiment_ed25519
    root: /home/experiment/dis-runs/two-host
placement:
  peers: [controller, worker, controller, worker, controller, worker]
  orderers: [controller, worker, controller]
~~~

A host without ssh represents the controller. A host with ssh is managed remotely. identity_file is optional when SSH already selects the correct key. root is a dedicated worker runtime directory, separate for each experiment. Node placement may use any number of configured hosts. If placement is omitted, nodes are assigned across hosts in round-robin order.

All advertised peer and orderer endpoints must be reachable from every machine. Certificates include the advertised addresses, and channel configuration uses them for Raft membership, anchor peers, and discovery. External chaincode connections, operations, and administration stay on each node's loopback interface. The controller also needs access to every peer endpoint for lifecycle management and acceptance checks.

Run the same commands as for a single-machine experiment, using the edited topology and a new DIS_RUNTIME. The manager transfers binaries, node configurations, and each worker's node identities over SSH, then starts and tracks the remote processes. Chaincode packages are shared within an organization; the external builder binds each peer to its colocated chaincode server. Redis, API, the event publisher, and CoreDNS run on the controller.

## Inspect and stop an experiment

~~~bash
python3 scripts/network.py status
python3 scripts/services.py status
python3 scripts/services.py down
python3 scripts/network.py down
~~~

Use the same DIS_RUNTIME on every command. The network manager stops its recorded local and remote processes and retains ledger data for restart. Select another runtime directory to deploy another scale. Topology selection occurs when creating a network; membership changes to a running channel are outside these commands.

## Verification and experiment records

acceptance.py --restart --all-gateways checks the signed workflow, every peer's committed state, discovery through every Gateway, explicit endorsement by each peer replica, DNS publication, exact request replay, and local/remote restart recovery. acceptance-events.py verifies checkpoint catch-up with the publisher stopped during commits.

Reports under DIS_RUNTIME record organization, peer, orderer, host, and endorsement counts together with verification results. Keep the topology and report with performance measurements so that organizational scale, peer replication, ordering membership, and physical machine placement are explicit.
