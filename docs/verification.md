# System verification

Verification covers business transitions, Fabric commits, DNS publication, and deployment recovery. Run connected checks against a dedicated deployment because they create records and may restart its components.

## Unit tests and static checks

~~~bash
go test -race ./...
go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py' -v
~~~

The tests exercise signatures, ownership, updates, request replay, ledger mappings, DNS encoding, recipient removal, event application, configuration validation, and process management.

## Connected workflow

Start the default configured deployment with ./run.sh, then select its runtime:

~~~bash
export DIS_RUNTIME="$PWD/runtime/configured"
export DIS_FABRIC_CONFIG="$DIS_RUNTIME/fabric/gateway.json"
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
go test -race -count=1 -v ./internal/fabric -run TestLiveFabricWorkflow
~~~

Acceptance checks signed registration and authorization, queries through every peer, Gateway discovery, replica endorsements, exact retries, complete DNS zones, and restart recovery. The event test stops the publisher during commits and checks subsequent catch-up. Reports and generated records are saved in DIS_RUNTIME.

## Management updates

~~~bash
DIS_TEST_DNS_ADDRESS=127.0.0.1:38530 \
go test -race -count=1 -v ./internal/fabric -run TestLiveFabricManagementUpdates
~~~

Set DIS_FABRIC_CONFIG as above. Adjust the DNS address for other configurations. This workflow covers address and signature changes, ownership transfer, and authorization status, permission, recipient, and expiration changes. See [management operations](management-updates.md).

## Redis integration

Use an isolated Redis instance; these tests replace the DIS keys in that instance. Serialize packages sharing the instance:

~~~bash
DIS_TEST_REDIS_ADDRESS=127.0.0.1:16379 go test -race -p 1 ./...
~~~

## Acceptance criteria

| Area | Expected result |
|---|---|
| Signed operations | Valid requests commit; invalid signatures and unauthorized changes are rejected |
| Retries | An identical actor/request pair returns its recorded result; changed content conflicts |
| Ledger queries | Configured peers return the committed business records |
| Publication | Redis state and complete authoritative zones match the ledger projection after catch-up |
| Recipient changes | Old authorization keys and DNS names are removed |
| Recovery | Restart preserves records; event processing resumes from the saved checkpoint |
| Deployment | Requested topology and ports appear in status; managed processes start and stop correctly |

Use the same checks for each topology selected in [deployment](experiment-deployment.md). Throughput and latency are measured separately with the [Caliper workload](../benchmarks/caliper/README.md).

## Ownership transfer and event recovery

The regression suite checks that a new owner can decide a pre-transfer pending request, the former owner is rejected, and the decision sets issued_at. It also checks first-time status updates of pending requests. The live Fabric regression is TestLiveFabricPendingTransferAndColdRecovery in internal/fabric/transfer_integration_test.go. Its cold replay is compared with the current ledger objects, including a history containing invalidated transactions.

GetSnapshot is tested with additional malformed receipt entries to ensure it neither reads nor returns request history. Recovery parsing tests distinguish valid DIS events, invalid transactions, and other chaincodes. Redis integration tests verify atomic records/checkpoint persistence, recovery, recipient moves, and rejection of conflicting or older publications. Configuration tests cover advertised nameserver addresses, while HTTP tests distinguish business conflict and retryable ledger-conflict responses.
