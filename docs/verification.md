# System verification

The connected reconstruction runs Fabric 2.4.9, Redis 7.2.5, and CoreDNS 1.12.0 with the repository's disredis plugin on Linux amd64.

## Verified behavior

- Signed registration, authorization requests, and owner decisions commit through Fabric.
- Forged signatures, non-owner decisions, and conflicting request IDs are rejected.
- Exact retries return their stored results without changing business sequence.
- Four peers independently return the expected authorization.
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
python3 scripts/acceptance.py --restart
python3 scripts/acceptance-events.py
```

An optional Redis integration test uses DIS_TEST_REDIS_ADDRESS pointing to an isolated instance. That test owns the DIS keys in the selected instance; use a separate test port.

Runtime acceptance reports contain concrete results. Generated identities, keys, ledgers, and logs remain outside version control. The [source mapping](original-implementation.md) identifies which upstream mechanisms are recovered and which interfaces remain adapted.

## Fresh deployment verification

A separate clean clone at revision 049b493 built all commands and Redis from the verified release archives. It created a new Fabric network with fresh certificates and ledgers, then passed the workflow, peer-query, exact-retry, complete DNS transfer, restart, and stopped-publisher catch-up checks. Only downloaded release archives were reused; no running deployment state was copied. The temporary network and services were stopped after verification.
