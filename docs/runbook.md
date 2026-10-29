# Running DIS-reconstruct

Run the signed registration and authorization workflow through the development API. The file backend persists single-process business state; the Fabric chaincode adapter uses the same transition rules.

## Requirements

- Git and Go 1.22 or later
- curl for HTTP checks
- A Linux host for the file backend's advisory lock
- An available TCP port (8080 by default)

## Get the source

```bash
git clone https://github.com/han-ry19/DIS-reconstruct.git
cd DIS-reconstruct
go test ./...
```

## Start the API

```bash
go run ./cmd/dis-api -listen 127.0.0.1:8080 -state runtime/state.json -domain dis-local
```

The state file and its lock are created automatically. Restarting with the same state path and domain preserves records and request receipts. Stop the API with Ctrl+C before opening that state file in another API process.

In another terminal:

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/healthz
```

Expected response:

```json
{"Status":"ok","backend":"file-development"}
```

## Run the business workflow

From the repository root:

```bash
go run ./cmd/dis-demo -url http://127.0.0.1:8080 -domain dis-local
```

The demo generates temporary user and application identities, registers data, requests Read permission, records the owner's approval, and queries the approved authorization. Each run uses fresh identifiers. Its private keys remain in the demo process.

## Client key generation and signing

```bash
go run ./cmd/dis-cli keygen -out identity.key
go run ./cmd/dis-cli sign -key identity.key -request request.json > signed-request.json
curl --fail-with-body -H 'Content-Type: application/json' --data-binary @signed-request.json http://127.0.0.1:8080/v1/transactions
```

Key generation prints the Base64 public key and creates a private-key file with owner-only permissions. The signing command reads an unsigned request matching the [API schema](../api/openapi.yaml). Request timestamps use Unix seconds and a five-minute execution window. Exact retries return their recorded result, including after restart.

## Verification

```bash
go test -race ./...
go vet ./...
go build ./cmd/dis-api ./cmd/dis-cli ./cmd/dis-demo ./cmd/dis-chaincode
```

Tests cover the HTTP business workflow, signatures, permissions, conflicting requests, repeated submissions, concurrent registration, state rollback, file locking, restart recovery, and the Fabric shim adapter. Shim tests exercise the adapter through Fabric's test stub; network endorsement and commit require a deployed Fabric network.

## Component availability

The HTTP development backend, request signing, business workflow, and Fabric shim adapter are implemented. Fabric network deployment and the API's Fabric client connection are the next integration steps. DNS synchronization remains a service entry point, with publication setup to follow.

## Troubleshooting

A state-lock error indicates another process is using that state file. An address-in-use error indicates a conflict on the selected port; choose another with -listen and pass its URL to the demo. An invalid timestamp response requires a fresh signed request and a correctly configured host clock.
