# System verification

The connected reconstruction runs Fabric 2.4.9, Redis 7.2.5, and CoreDNS 1.12.0 with the repository's disredis plugin on Linux amd64.

## Verified behavior

- Signed registration, authorization requests, and owner decisions commit through Fabric.
- Forged signatures, non-owner decisions, and conflicting request IDs are rejected.
- Exact retries return their stored results without changing business sequence.
- Every configured peer independently returns the expected authorization.
- Every Gateway discovers the deployed endorsement policy; explicit replica endorsements commit successfully.
- Committed chaincode events publish the matching Redis and authoritative DNS state.
- Full TCP AXFR compares every record in both zones with the ledger projection.
- A 600-object zone transfers across multiple DNS messages without losing records.
- Fabric, Redis, API, publisher, and CoreDNS restart with records retained.
- A publisher stopped during a complete business workflow catches up from its event checkpoint.
- The v1 state document migrates to object keys and preserves historical receipts.
- Race-enabled Go tests and static analysis pass.

## Commands

```bash
go test -race ./...
go vet ./...
DIS_FABRIC_CONFIG="$PWD/runtime/fabric/gateway.json" go test -v ./internal/fabric -run TestLiveFabricWorkflow
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
```

An optional Redis integration test uses DIS_TEST_REDIS_ADDRESS pointing to an isolated instance. That test owns the DIS keys in the selected instance; use a separate test port.

Runtime acceptance reports contain concrete results. Generated identities, keys, ledgers, and logs remain outside version control. The [source mapping](original-implementation.md) connects the implemented registration, authorization, and resolution components to their upstream source references.

## Fresh deployment verification

A separate clean clone at revision 049b493 built all commands and Redis from the verified release archives. It created a new Fabric network with fresh certificates and ledgers, then passed the workflow, peer-query, exact-retry, complete DNS transfer, restart, and stopped-publisher catch-up checks. Only downloaded release archives were reused; no running deployment state was copied. The temporary network and services were stopped after verification.

## Configurable deployment verification

The following connected deployments passed the signed workflow, all-peer queries, all-Gateway discovery, explicit replica endorsement, complete DNS verification, exact replay, and component restart checks:

| Organizations | Peers per organization | Orderers | Endorsement threshold | Machines |
|---|---|---|---|---|
| 4 | 1 | 4 | 3 | 1 |
| 3 | 2 | 3 | 2 | 1 |
| 5 | 1 | 3 | 3 | 1 |
| 3 | 2 | 3 | 2 | 2 |

The two-machine deployment also passed stopped-publisher catch-up. Existing four-organization ledgers restarted with their original ports and data. Each temporary deployment used an isolated runtime directory. These checks verify deployment and business behavior; throughput and latency measurements are separate experiments.

Topology validation, legacy address preservation, remote path rendering, and stale-PID process protection are covered by:

~~~bash
python3 -m unittest discover -s scripts -p 'test_*.py' -v
~~~
