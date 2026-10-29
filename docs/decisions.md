# Implementation decisions

## Deterministic data registration

DIA's registration algorithm creates a data identifier using a UUID. In this reconstruction, the client supplies a UUID candidate bound to the request. The contract checks its format and uniqueness so endorsing peers can produce consistent results from the same input.

## Owner decisions and Fabric endorsement

The data owner decides an application's access permissions. Fabric peers check requester identity, permissions, and state conditions under the contracts and endorse the execution result. The API and transaction workflow preserve these distinct responsibilities.

## Consistent time inputs

Issuance and expiration fields use inputs shared within the transaction. Their source and validation rules are to be specified with the authorization implementation so peers interpret the same record consistently.

## Ledger-to-DNS synchronization

Following the periodic synchronization approach in the papers, the publication service reads committed records and generates DNS zone data. Operational checks distinguish ledger results from DNS responses.

## Business scope

The reconstruction covers identity registration and lookup, data registration and resolution, and authorization requests and decisions. Data addresses connect these records to external data services. Interfaces, configuration, and acceptance scenarios follow this workflow.
