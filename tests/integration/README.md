# Integration verification

Go integration tests connect the business rules to Fabric and the DNS projection to Redis. Set DIS_FABRIC_CONFIG for live ledger tests and DIS_TEST_DNS_ADDRESS to verify authoritative publication during management updates.

~~~bash
export DIS_FABRIC_CONFIG="$PWD/runtime/configured/fabric/gateway.json"
DIS_TEST_DNS_ADDRESS=127.0.0.1:38530 \
go test -race -count=1 -v ./internal/fabric
~~~

Use a dedicated deployment. Redis tests use DIS_TEST_REDIS_ADDRESS and replace that instance's DIS keys. Run packages serially when sharing one Redis instance:

~~~bash
DIS_TEST_REDIS_ADDRESS=127.0.0.1:16379 go test -race -p 1 ./...
~~~

See [system verification](../../docs/verification.md) for setup and acceptance criteria.
