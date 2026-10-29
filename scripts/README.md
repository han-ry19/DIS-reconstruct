# Operational scripts

The process smoke test starts an API on an available loopback port, runs the signed demo, submits a CLI-signed registration, restarts the API, and verifies persisted records. It uses temporary state and stops its processes on completion.

Build the binaries and run the test from the repository root (Python 3 is required):

```bash
go build -o bin/ ./cmd/dis-api ./cmd/dis-cli ./cmd/dis-demo
python3 scripts/smoke.py
```

See the [running guide](../docs/runbook.md) for interactive use. Network deployment and DNS lifecycle scripts will accompany those integrations.
