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

## Ledger schema

The current chaincode uses ledger schema 4, with independent authorization records and indexed queries. Use a fresh runtime directory when starting from an earlier schema; retain the earlier runtime separately. The launcher rejects a mismatched schema. See [ledger and DNS implementation](../../docs/original-implementation.md).
