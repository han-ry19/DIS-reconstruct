# System verification

The reconstruction has passed an end-to-end deployment using the committed source, official Fabric 2.4.9 binaries, and CoreDNS 1.12.0 on Linux amd64.

## Verified configuration

- Four Fabric peer organizations, with one peer per organization.
- Four Raft orderers and the dis application channel.
- A three-of-four organization endorsement policy for the dis chaincode.
- Signed management requests through the Fabric-backed API.
- Periodic publication of committed state into the uid and data authoritative DNS zones.

## Acceptance results

| Check | Result |
|---|---|
| User and application registration, data registration, authorization request, and owner approval | Passed |
| Authorization queries against all four peers | Passed |
| Exact retries return their original results without advancing business state | Passed |
| Forged signatures, decisions by a non-owner, and conflicting request IDs are rejected | Passed |
| Full TCP AXFR of both DNS zones matches the committed snapshot | Passed |
| API, synchronizer, CoreDNS, peers, chaincode services, and orderers restart with state retained | Passed |
| Original signed requests remain replayable after restart without repeating their effects | Passed |
| Go race-enabled tests, static analysis, and the live Fabric integration test | Passed |

## Fresh-checkout reproduction

A separate clean clone was built from commit 1a57199 with no existing runtime directory or application binaries. The bootstrap script installed the pinned official releases from cached archives and verified their SHA-256 hashes. The network manager generated new certificates, created new ledgers, installed and initialized the chaincode, and configured organization anchors. The complete restart acceptance and live Fabric tests then passed against that deployment.

The test used cached release archives. It verifies installation and deployment independently of earlier runtime state; downloading those archives depends on the host's access to GitHub releases.

## Reproduce the checks

Follow the [running guide](runbook.md) to install, build, and start the system, then run:

```bash
go test -race ./...
go vet ./...
python scripts/acceptance.py --restart
DIS_FABRIC_CONFIG="$PWD/runtime/fabric/gateway.json" go test -v ./internal/fabric -run TestLiveFabricWorkflow
```

The acceptance runner writes its results to runtime/acceptance.json. Runtime files, generated identities, private keys, ledgers, and logs remain outside version control.
