# Implementation decisions

## Deterministic data registration

DIA's registration algorithm creates a data identifier using a UUID. In this reconstruction, the client supplies a UUID candidate bound to the request. The contract checks its format and uniqueness so endorsing peers can produce consistent results from the same input.

## Owner decisions and Fabric endorsement

The data owner decides an application's access permissions. Fabric peers check requester identity, permissions, and state conditions under the contracts and endorse the execution result. The API and transaction workflow preserve these distinct responsibilities.

## Consistent time inputs

Issuance and expiration fields use inputs shared within the transaction. The development API uses its execution clock; the Fabric adapter uses GetTxTimestamp. This Fabric timestamp is transaction input, rather than an independently certified wall clock. New requests must fall within five minutes of the execution timestamp; exact recorded retries retain their original result.

## Ledger-to-DNS synchronization

Following the periodic synchronization approach in the papers, the publication service reads committed records and generates DNS zone data. Operational checks distinguish ledger results from DNS responses.

## Business scope

The reconstruction covers identity registration and lookup, data registration and resolution, and authorization requests and decisions. Data addresses connect these records to external data services. Interfaces, configuration, and acceptance scenarios follow this workflow.

## Request authentication and retries

Ed25519 signatures bind the domain, operation, actor, request ID, timestamp, and typed parameters. SigningBytes in internal/domain defines the versioned encoding. A request ID is scoped to its actor: an exact retry returns the stored result, and a different payload with the same ID is rejected.

Data signatures are stored as data attributes and checked for encoding. Verification against the original data occurs in the data-consumption workflow.

## Authorization record updates

Following DIA Algorithm 2, a newly signed request writes a pending record at the data/owner/recipient composite key. An exact retry returns its historical result. Following Algorithm 3, an owner decision preserves requested permissions and expiration while setting status and issuance time.

## Initial state layout

The development backend persists an atomic JSON state document and uses a single-writer file lock. The initial Fabric adapter stores the document at dis.state.v1 and uses GetState/PutState for the transition. This layout gives a common read/write dependency to updates; per-record ledger keys are an integration task before concurrency performance evaluation.
