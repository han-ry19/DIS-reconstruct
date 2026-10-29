# Integration verification

Integration tests will check the boundaries between requests, contracts, ledger state, and publication:

- Request signatures and permissions are checked through the API and contract path.
- Valid transactions produce the expected management records.
- Conflicting and repeated requests follow the defined business semantics.
- Published DNS records match committed state.

Test inputs and expected outcomes will follow the business contracts as the components are connected.
