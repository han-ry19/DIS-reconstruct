# Fabric network

The default native Linux deployment runs four peer organizations, one peer per organization, and four Raft orderers in a separate ordering organization. Topology files configure other organization, peer, orderer, endorsement, and machine counts; see the [experiment deployment guide](../../docs/experiment-deployment.md). The application channel and chaincode are named dis. The business endorsement policy requires any three of the four peer organizations.

Peer and orderer traffic uses TLS. Each peer connects to its own external chaincode process on loopback. Generated development MSP identities and durable ledgers are stored under runtime/fabric.

## Setup

Requirements: Linux amd64, Go 1.23.4 or later, Python 3.9 or later, and PyYAML.

```bash
python3 -m pip install PyYAML
python3 scripts/bootstrap.py
go build -o bin/ ./cmd/...
python3 scripts/network.py up
```

The bootstrap script downloads the official Fabric 2.4.9 release. A previously downloaded archive can be supplied with --archive and an expected checksum with --sha256. FABRIC_CFG_PATH is set by the network manager for each component.

On a new deployment, --port-base or the topology file selects the Fabric port range. Peer and orderer endpoints are advertised using the configured host addresses. External chaincode, operations, and administration endpoints use loopback. The manager writes logs, certificates, ledger data, and gateway.json beneath DIS_RUNTIME/fabric; DIS_RUNTIME defaults to runtime in the checkout.

## Run the API and demo

```bash
bin/dis-api -fabric-config runtime/fabric/gateway.json
```

In another terminal:

```bash
bin/dis-demo
DIS_FABRIC_CONFIG="$PWD/runtime/fabric/gateway.json" go test -v ./internal/fabric -run TestLiveFabricWorkflow
```

The API waits for valid Fabric commit before returning a successful transaction result. The live integration test submits business operations and checks signatures, owner permissions, conflicting request IDs, retries, and query results.

## Stop and restart

```bash
python3 scripts/network.py status
python3 scripts/network.py down
python3 scripts/network.py up
```

The manager restarts the saved network using its existing certificates and ledgers. Stop the API separately with Ctrl+C. The manager controls only the process IDs and start identities recorded for this project.

The four-organization topology is a reconstruction configuration; the paper-to-implementation mapping is documented separately. The initial chaincode state layout is described in [implementation decisions](../../docs/decisions.md).
