# Running DIS-reconstruct

This guide runs the Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications registration and authorization workflow through Fabric and publishes the resulting records through CoreDNS.

## Requirements

- Linux amd64
- Git and Go 1.23.4 or later
- A C compiler and make for Redis
- Python 3.9 or later, with venv support
- Access to GitHub releases and the Go module proxy for dependency downloads
- Available ports for the chosen Fabric topology, plus the API, DNS, and Redis service ports

## Install and build

```bash
git clone https://github.com/han-ry19/DIS-reconstruct.git
cd DIS-reconstruct
python3 -m venv .venv
. .venv/bin/activate
python -m pip install PyYAML
python scripts/bootstrap.py
go build -o bin/ ./cmd/...
```

The bootstrap script installs Fabric 2.4.9, the CoreDNS 1.12.0 release archive, and Redis 7.2.5 under runtime/tools. The build command creates dis-coredns with the Redis plugin used by the connected deployment. Offline release archives can be provided with --archive, --coredns-archive, and --redis-archive. See the [Fabric setup](../deploy/fabric/README.md) for network details.

## Start the system

```bash
python scripts/network.py up
python scripts/services.py up
```

By default, the network manager creates four peer organizations, one peer per organization, four Raft orderers, the dis channel, and the dis chaincode. Any three of the four peer organizations satisfy the business endorsement policy. The service manager starts the API, committed Fabric event publication to Redis, and CoreDNS.

For other node counts and deployments across machines, use a topology file and a separate DIS_RUNTIME as described in the [experiment deployment guide](experiment-deployment.md).

## Run the workflow

```bash
bin/dis-demo -url http://127.0.0.1:28080
bin/dis-sync -fabric-config runtime/fabric/gateway.json -redis 127.0.0.1:28679 -check -verify 127.0.0.1:28530
```

The demo creates user and application identities, registers data, requests Read access, records the owner's approval, and queries the authorization. Each run uses fresh identifiers. The verification command checks both authoritative DNS zones against the committed snapshot.

## Inspect, stop, and restart

```bash
python scripts/services.py status
python scripts/network.py status
python scripts/services.py down
python scripts/network.py down
python scripts/network.py up
python scripts/services.py up
```

Certificates, ledgers, published records, and logs are retained under runtime. Restart uses the saved configuration and records. Managers control only their recorded project processes. Logs are stored under runtime/fabric and runtime/services.

## Verification

```bash
go test -race ./...
go vet ./...
python scripts/acceptance.py --restart --all-gateways
python scripts/acceptance-events.py
DIS_FABRIC_CONFIG="$PWD/runtime/fabric/gateway.json" go test -v ./internal/fabric -run TestLiveFabricWorkflow
```

The live test exercises real Fabric commits and queries, signature rejection, owner permissions, request conflicts, and retries. Unit and integration tests also cover deterministic transitions, file-store recovery, publication encoding, and stale-publication rejection.

## Upgrade an existing v1 deployment

Build the commands, stop the services and network, and run python scripts/upgrade-ledger.py. The upgrade script preserves the previous publication status under runtime/reconstruction-backup. Start services again after migration. See the [source mapping](original-implementation.md) for migration behavior.

## Development without a Fabric network

The file backend provides a single-process environment for business development:

```bash
bin/dis-api -listen 127.0.0.1:8080 -state runtime/state.json -domain dis-local
```

In another terminal:

```bash
bin/dis-demo -url http://127.0.0.1:8080
```

## Signing a request

```bash
bin/dis-cli keygen -out identity.key
bin/dis-cli sign -key identity.key -request request.json > signed-request.json
curl --fail-with-body -H 'Content-Type: application/json' --data-binary @signed-request.json http://127.0.0.1:28080/v1/transactions
```

Key generation prints the public key and writes a private key with owner-only permissions. Requests follow the [API schema](../api/openapi.yaml). Exact retries return the original result; reuse of the same actor/request ID for a different payload is rejected.

## Troubleshooting

An address-in-use error requires another initial port base or stopping the conflicting service. A state-lock error means another process is using that development state file. A timestamp error requires a fresh signed request and a correctly configured clock.

If a component exits, inspect its log before restarting. Stop the services before stopping the Fabric network. For an interrupted initial deployment, inspect the saved phase and commands.log under runtime/fabric.
