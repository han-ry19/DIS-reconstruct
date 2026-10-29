# Integration verification

Integration tests check the boundaries between requests, contracts, ledger state, and publication:

- Request signatures and permissions are checked through the API and contract path.
- Valid transactions produce the expected management records.
- Conflicting and repeated requests follow the defined business semantics.
- Published DNS records match committed state.

The HTTP tests in internal/api cover the business workflow, failure cases, concurrency, and restart recovery. The tests in chaincode/dis exercise the Fabric adapter using the shim test stub. DNS publication checks will follow synchronization implementation.
