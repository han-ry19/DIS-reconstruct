# Implementation decisions

## Deterministic data registration

DIA's registration algorithm creates a data identifier using a UUID. In this reconstruction, the client supplies a UUID candidate bound to the request. The contract checks its format and uniqueness so endorsing peers can produce consistent results from the same input.

## Owner decisions and Fabric endorsement

The data owner decides an application's access permissions. Fabric peers check requester identity, permissions, and state conditions under the contracts and endorse the execution result. The API and transaction workflow preserve these distinct responsibilities.

## Consistent time inputs

Issuance and expiration fields use inputs shared within the transaction. The development API uses its execution clock; the Fabric adapter uses GetTxTimestamp. This Fabric timestamp is transaction input, rather than an independently certified wall clock. New requests must fall within five minutes of the execution timestamp; exact recorded retries retain their original result.

## Ledger-to-DNS synchronization

Following the original service and chaincode, a full snapshot initializes the projection and committed chaincode events update Redis records. CoreDNS reads those records through disredis. Operational checks distinguish ledger results from authoritative DNS responses.

## Business scope

The reconstruction covers identity registration and lookup, data registration and resolution, and authorization requests and decisions. Data addresses connect these records to external data services. Interfaces, configuration, and acceptance scenarios follow this workflow.

## Request authentication and retries

Ed25519 signatures bind the domain, operation, actor, request ID, timestamp, and typed parameters. SigningBytes in internal/domain defines the versioned encoding. A request ID is scoped to its actor: an exact retry returns the stored result, and a different payload with the same ID is rejected.

Data signatures are stored as data attributes and checked for encoding. Verification against the original data occurs in the data-consumption workflow.

## Authorization record updates

Following DIA Algorithm 2, a newly signed request writes a pending record at the data/owner/recipient composite key. An exact retry returns its historical result. Following Algorithm 3, an owner decision preserves requested permissions and expiration while setting status and issuance time.

## Initial state layout

The development backend persists an atomic JSON state document and uses a single-writer file lock. Fabric stores identities, data objects, pending authorizations, and receipts under composite keys. Confirmed authorizations are embedded in each data object, following the original chaincode. The reconstruction retains a shared metadata sequence for compatibility with results and DNS serials. MigrateState converts an existing dis.state.v1 document in place. See the original implementation mapping for concurrency and interface boundaries.

## DNS record encoding

The projection follows Dominate's identity CERT/TXT, data RP/URI/TXT, and authorization TXT structure. For Ed25519 key material, CERT type 254 carries an OID-length byte, the DER OID value for 1.3.101.112, and the raw key. Data profiles use a separate _profile TXT name. Full AXFR verification compares all authoritative records, including SOA serials, with the committed snapshot.

## Completion timestamps

The result field committed_at records the execution timestamp used by the transition. In Fabric it is the transaction timestamp; a successful HTTP result additionally requires a valid commit status. It is not an instrumented peer database commit time.
