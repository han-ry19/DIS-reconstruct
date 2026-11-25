# Fabric network

The network manager creates peer organizations, Raft orderers, the dis channel, and the dis chaincode. The default configuration uses four organizations with one peer each and requires three organizations to endorse a business operation.

## Start and inspect

Install dependencies and build commands using the [running guide](../../docs/runbook.md), then run:

~~~bash
./run.sh plan
./run.sh
./run.sh status
~~~

[config.yaml](../../config.yaml) controls organization counts, peer replicas, ordering nodes, endorsement threshold, Gateway concurrency, addresses, and placement. See [deployment](../../docs/experiment-deployment.md) for multiple machines.

## Runtime layout

Under the configured runtime directory, fabric/ stores certificates, node configuration, ledgers, process records, logs, and gateway.json. Peer and orderer traffic uses TLS. Each peer connects to its colocated external chaincode process on loopback. Fabric's organizational identities and the end-user request signatures serve separate roles: network participation and business authorization.

The API uses gateway.json to submit transactions and waits for valid commit status. Contracts verify the embedded business request and update the ledger.

## Component-level commands

For the default unified configuration:

~~~bash
export DIS_RUNTIME="$PWD/runtime/configured"
python3 scripts/network.py status
~~~

Use ./run.sh down and ./run.sh to stop and restart the complete deployment. For manual operations, stop services before network nodes. The network manager controls recorded process identities and retains ledger data on shutdown.

## Legacy ledger migration

For a deployment using the earlier single-document ledger layout, select its DIS_RUNTIME, stop services and network, build the current commands, then run python3 scripts/upgrade-ledger.py. The script advances the chaincode definition and converts the state while preserving identifiers and receipts. Start the complete deployment again after migration. See [source and storage design](../../docs/original-implementation.md).
