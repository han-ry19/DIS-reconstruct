# Data and authorization management

The management path implements the data and authorization update operations in Section 5.1.5 of **Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications**. Algorithm 7 appears on PDF page 19 and Algorithm 8 on page 20. The supporting discussion begins on page 18. Registration and owner decisions also follow Algorithms 1–3 of **Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership**, PDF pages 4–5.

## Signed operations

Submit updates through POST /v1/transactions using the same signed request envelope as registration. The actor is the current data owner. The signature includes field and new_content. Each operation changes one field; changing several fields requires several transactions, as in the paper algorithms.

| Operation | field | new_content | Effect |
|---|---|---|---|
| data.update | URI | Absolute address with a scheme and host | Replace the stored data address |
| data.update | TXT | Base64-encoded 64-byte data signature | Replace the signature attribute |
| data.update | RP | Registered user identity | Transfer ownership |
| authorization.update | status | approved or rejected | Change the existing authorization decision |
| authorization.update | permissions | JSON array encoded as a string, for example ["Read","Update"] | Replace permissions |
| authorization.update | recipient | Registered application identity | Move the authorization to the new recipient |
| authorization.update | expires_at | Positive Unix seconds encoded as a string | Replace expiration; a past value expires the grant |

Data updates require data_id. Authorization updates also require recipient, identifying the existing authorization. The contract selects the unique (data, recipient) record, then checks the current data owner.

A signed request uses a fresh request_id. Retrying the exact request returns the recorded result. Reusing that request ID with changed signed content returns a conflict.

For example, the parameters for changing an address are:

~~~json
{
  "data_id": "550e8400-e29b-41d4-a716-446655440000.data",
  "field": "URI",
  "new_content": "https://data.example/updated"
}
~~~

The parameters for reducing permissions are:

~~~json
{
  "data_id": "550e8400-e29b-41d4-a716-446655440000.data",
  "recipient": "app.uid",
  "field": "permissions",
  "new_content": "[\"Read\"]"
}
~~~

Put these parameters in a request with operation data.update or authorization.update, the deployment domain, current owner as actor, a fresh request_id, and the current Unix timestamp. Sign the complete request using dis-cli sign and submit it as described in the [running guide](runbook.md). The [OpenAPI schema](../api/openapi.yaml) defines the envelope.

## Input limits

Limits are measured in UTF-8 bytes. Addresses accept up to 1024 bytes for both registration and URI updates. Profile label, format, and description accept up to 256, 128, and 4096 bytes. Oversized fields are rejected before state changes. The address bound fits the URI encoder used by the pinned DNS library.

## Ownership and recipient changes

An RP update changes the data owner and preserves existing authorization records, consistent with Algorithm 7's single-field update. The former owner cannot make subsequent changes. The new owner can decide an existing pending request or update a grant using data_id and recipient. The original issuer remains in owner_id. Decisions set issued_at; changing a pending record to approved or rejected through a status update also sets issued_at.

DNS authorization names contain the recipient and data identifier. Changing recipient removes the old authorization key and DNS label before publishing the replacement. A collision with another confirmed or pending authorization is rejected, including authorizations retained from a previous owner. An application cannot create a second issuer-specific grant at the same DNS name after ownership transfer; the current owner can edit the existing record.

Only the first application request creates the data/recipient authorization record. Further requests return HTTP 409 while that record exists, regardless of its status, expiration, or original issuer. Rejected and expired records remain available for the current owner to update. Exact retries return the saved result and never reset an owner decision. A recipient update moves the record, removing the old name; the old name can subsequently receive a new request if no record remains there. Non-status field updates preserve the existing status and issuance time. A status update sets issuance time when first finalizing a pending record; later status updates retain that time. The owner-decision operation sets issuance time when deciding a request.

## Ledger records and queries

The reconstruction maintains these auxiliary ledger records:

| Key family | Role |
|---|---|
| DOIToIndex / IndexToDOI | Reversible identifier mappings; raw byte values |
| dataOwner-dataObject | Owner-to-data association |
| DOAuthorized | Recipient-to-data association |
| TransactionInfo | Data and identity transaction information |
| AuthorizationTransactionInfo | Per-data/recipient authorization transaction information |
| RegistrationData | Registration description |

The internal index encodes the transaction ID and identifier using Base64. New registrations write mappings, associations, transaction information, and registration metadata. Updates maintain the affected associations and transaction information. Timestamps use Unix seconds.

GetTransactionInfo combines object and per-recipient transaction information when queried; management writes do not update a shared latest-transaction record. It selects the greatest execution timestamp (seconds and nanoseconds), with transaction ID as a deterministic tie-breaker. This is execution-time metadata, not a global commit-order counter.

The contract exposes TransDOIToIndex, TransIndexToDOI, GetTransactionInfo, GetOwnedData, and GetAuthorizedData, each taking one identifier (TransIndexToDOI takes an internal index). The list queries validate authoritative business records, with approved status and unexpired authorization required for recipient access. Queries use the owner's or recipient's association index and validate each referenced business record, including status and expiration.

## Event publication

Each successful update emits the affected business object. Recipient moves also emit the removed authorization identity. The publisher applies this deletion and the replacement to its state, then publishes Redis DNS fields, cached business objects, and checkpoint together. Replaying the event does not restore removed records. Full-snapshot and incremental publication generate the same records.

The DNS projection publishes identity CERT/TXT, data RP/URI/TXT, and authorization TXT. Profiles remain registration metadata. External data services retain the payload; a signature update records the owner-supplied signature attribute.

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
DIS_FABRIC_CONFIG="$PWD/runtime/configured/fabric/gateway.json" \
DIS_TEST_DNS_ADDRESS=127.0.0.1:38530 \
go test -race -count=1 -v ./internal/fabric -run TestLiveFabricManagementUpdates
~~~

The unit tests cover all seven fields, rejected changes with unchanged state, ownership transfer, collisions, first-request creation and rejection of repeat applications, exact retries, HTTP persistence, event/snapshot equivalence, Redis key deletion, and recovery. The integration workflow commits fourteen transactions and checks all seven update fields.

## Concurrent authorization verification

The isolated read/write-set test checks that different-recipient operations have no shared writes or range reads. It also checks conflicts for the same authorization, recipient collisions, and ownership changes. The live Fabric test endorses all requests in each group before submitting any of them, then verifies every commit status:

~~~bash
DIS_FABRIC_CONFIG="$DIS_RUNTIME/fabric/gateway.json" go test ./internal/fabric -run TestLiveFabricIndependentAuthorizations -count=1 -v
~~~

It covers four recipients of one data object for requests, decisions, and updates. Separate cases verify MVCC rejection of conflicting same-authorization updates and an operation endorsed before an ownership transfer.

Authorization lookup uses GET /v1/authorizations?data_id=ID&recipient=APP and the two-argument chaincode query GetAuthorization(dataID, recipient). The original issuer in owner_id is response metadata, not a lookup filter. Existing HTTP clients may continue sending owner_id; it is ignored.
