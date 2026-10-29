# Operational scripts

The process smoke test starts an API on an available loopback port, runs the signed demo, submits a CLI-signed registration, restarts the API, and verifies persisted records. It uses temporary state and stops its processes on completion.

Build the binaries and run the test from the repository root (Python 3 is required):

```bash
go build -o bin/ ./cmd/dis-api ./cmd/dis-cli ./cmd/dis-demo
python3 scripts/smoke.py
```

See the [running guide](../docs/runbook.md) for interactive use. Network deployment and DNS lifecycle scripts will accompany those integrations.

## Connected system acceptance

After starting the Fabric network and connected services, run:

    python3 scripts/acceptance.py --restart

The script verifies a signed business workflow, queries the authorization on all four peers, checks both authoritative DNS zones, restarts the managed components, and repeats the record checks. Results are saved to runtime/acceptance.json. The system is left running for inspection.
