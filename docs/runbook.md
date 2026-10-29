# Running DIS-reconstruct

This guide explains how to obtain the source, check the Go packages, and start the available API endpoint.

## Requirements

- Git
- Go 1.22 or later
- curl for the health check
- An available TCP port 8080

## Get the source

```bash
git clone https://github.com/han-ry19/DIS-reconstruct.git
cd DIS-reconstruct
```

Run the following commands from the repository root.

## Check the packages

```bash
go test ./...
```

This checks that the current packages compile. Automated test cases will accompany the business implementation.

## Start the API

```bash
go run ./cmd/dis-api
```

The API listens on port 8080. Keep this terminal running and open another terminal for the health check:

```bash
curl --fail --silent --show-error http://localhost:8080/healthz
```

Expected response:

```json
{"Status":"ok"}
```

Stop the API with Ctrl+C.

## Available functionality

The current executable API provides the health endpoint. Identity, data, and authorization models and the repository interface define the business objects. The CLI, synchronization service, and chaincode directory provide entry points for the business implementation.

Fabric deployment, management transactions, and CoreDNS publication will receive runnable instructions as those components are connected. The [architecture](architecture.md) describes their intended interaction.

## Troubleshooting

If startup reports that port 8080 is already in use, stop the conflicting service or use an environment where the port is available.

If Go reports an unsupported module version, check your active installation with the following command:

```bash
go version
```
