# Data and authorization management

The management path implements the data and authorization update operations in Section 5.1.5 of **Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications**. Algorithm 7 appears on PDF page 19 and Algorithm 8 on page 20. The supporting discussion begins on page 18. Registration and owner decisions also follow Algorithms 1–3 of **Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership**, PDF pages 4–5.

## Signed operations

Submit updates through POST /v1/transactions using the same signed request envelope as registration. The actor is the current data owner. The signature includes field, new_content, and authorization_owner. Each operation changes one field; changing several fields requires several transactions, as in the paper algorithms.

| Operation | field | new_content | Effect |
|---|---|---|---|
| data.update | URI | Absolute address with a scheme and host | Replace the stored data address |
| data.update | TXT | Base64-encoded 64-byte data signature | Replace the signature attribute |
| data.update | RP | Registered user identity | Transfer ownership |
| authorization.update | status | approved or rejected | Change the existing authorization decision |
| authorization.update | permissions | JSON array encoded as a string, for example ["Read","Update"] | Replace permissions |
| authorization.update | recipient | Registered application identity | Move the authorization to the new recipient |
| authorization.update | expires_at | Positive Unix seconds encoded as a string | Replace expiration; a past value expires the grant |

Data updates require data_id. Authorization updates also require recipient, identifying the existing authorization. authorization_owner selects its original issuer and defaults to the current data owner.

A signed request uses a fresh request_id. Retrying the exact request returns the recorded result. Reusing that request ID with changed signed content returns a conflict.

For example, the parameters for changing an address are:

~~~json
{
  "data_id": "sample.data",
  "field": "URI",
  "new_content": "https://data.example/updated"
}
~~~

The parameters for reducing permissions are:

~~~json
{
  "data_id": "sample.data",
  "recipient": "app.uid",
  "field": "permissions",
  "new_content": "[\"Read\"]"
}
~~~

Put these parameters in a request with operation data.update or authorization.update, the deployment domain, current owner as actor, a fresh request_id, and the current Unix timestamp. Sign the complete request using dis-cli sign and submit it as described in the [running guide](runbook.md). The [OpenAPI schema](../api/openapi.yaml) defines the envelope.

## Ownership and recipient changes

An RP update changes the data owner and preserves existing authorization records, consistent with Algorithm 7's single-field update. The former owner cannot make subsequent changes. The new owner can manage an existing grant by naming its original authorization_owner. This parameter addresses the reconstruction's existing data/issuer/recipient storage key; it does not create another governance role.

DNS authorization names contain the recipient and data identifier. Changing recipient removes the old authorization key and DNS label before publishing the replacement. A collision with another confirmed or pending authorization is rejected, including authorizations retained from a previous owner. An application cannot create a second issuer-specific grant at the same DNS name after ownership transfer; the current owner can edit the existing record.

A pending request can overlay an earlier decision at the same composite key. Updating or moving that pending record also removes the shadowed confirmed entry when necessary, so a snapshot or restart cannot resurrect the previous grant. Non-status field updates preserve the existing status and issuance time; changing status through the field-update operation also leaves the issuance attribute unchanged. The owner-decision operation sets issuance time when deciding a request.

## Ledger records and queries

The historical chaincode revision 63c85ef11515e9183e7e2bc4b01fb34f1e390d00 provides the source for the following key families:

| Key family | Role | Historical source |
|---|---|---|
| DOIToIndex / IndexToDOI | Reversible identifier mappings; raw byte values | contract/doi_index.go |
| dataOwner-dataObject | Owner-to-data association | contract/save_dataobject.go |
| DOAuthorized | Recipient-to-data association | contract/save_dataobject.go and contract/save_confirmed_auth.go |
| TransactionInfo | Creation time, latest transaction ID and update time | contract/get_transaction_info.go |
| RegistrationData | Registration description | contract/save_dataobject.go |

The internal index follows the historical Base64(transaction ID + identifier) construction. New registrations write mappings, associations, transaction information, and the profile's registration metadata. Updates move affected associations and retain creation time while recording the new transaction ID and timestamp. The current reconstruction retains its existing business-object schema; these restored records do not constitute a byte-compatible import of the historical ledger. Its timestamps use the current schema's Unix seconds.

The contract exposes TransDOIToIndex, TransIndexToDOI, GetTransactionInfo, GetOwnedData, and GetAuthorizedData, each taking one identifier (TransIndexToDOI takes an internal index). The list queries validate authoritative business records, with approved status and unexpired authorization required for recipient access. An association alone is not permission. Currently the list queries scan the business snapshot, keeping pre-index ledgers readable. Existing ledgers acquire auxiliary records as their objects are written; no full historical-index backfill is performed.

## Event publication

Each successful update emits the affected business object. Recipient moves also emit the removed authorization identity. The publisher applies this deletion and the replacement to its state, then publishes Redis DNS fields, cached business objects, and checkpoint together. Replaying the event does not restore removed records. Full-snapshot and incremental publication generate the same records.

The DNS projection publishes identity CERT/TXT, data RP/URI/TXT, and authorization TXT. Profiles remain registration metadata and are not published at an extra _profile DNS label. External payloads remain with their data service; updating a signature records the owner's supplied signature and does not fetch or verify that payload.

A valid Fabric commit completes the management transaction. The matching authoritative answer completes DNS publication. Recursive resolver caches follow the configured DNS TTL; this reconstruction implements the papers' DNS publication model.

## Tests

~~~bash
go test -race ./...
go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py' -v
~~~

For Redis tests, point DIS_TEST_REDIS_ADDRESS at an isolated instance and serialize packages because the test packages own the same DIS keys:

~~~bash
DIS_TEST_REDIS_ADDRESS=127.0.0.1:16379 go test -race -p 1 ./...
~~~

A running deployment of this chaincode enables the management integration test. The DNS address is optional; setting it additionally checks authoritative publication of each kind of update and removal of the former recipient:

~~~bash
DIS_FABRIC_CONFIG="$PWD/runtime/fabric/gateway.json" \
DIS_TEST_DNS_ADDRESS=127.0.0.1:28530 \
go test -race -count=1 -v ./internal/fabric -run TestLiveFabricManagementUpdates
~~~

The unit tests cover all seven fields, rejected changes with unchanged state, ownership transfer, collisions, pending/confirmed overlays, exact retries, HTTP persistence, event/snapshot equivalence, Redis key deletion, and recovery. The integration workflow commits fourteen transactions and checks all seven update fields.

## Reconstruction compatibility

The pre-existing Ed25519 request envelope, actor/request receipts, five-minute input window, receipt-derived publication sequence, and transactional Redis checkpoints remain in use. They are reconstruction engineering choices, not additional paper algorithms. Client-selected data names remain supported; the contract does not enforce UUIDv4. Identity rotation, Orb, discovery services, and external data delivery are outside this management completion.
