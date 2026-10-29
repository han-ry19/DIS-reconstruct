# Architecture

DIS-reconstruct follows the identifier-management workflow described in DIA and Dominate: register identities, register data, request access, record the owner's decision, and resolve the resulting records.

## Application interface

The client and DIS API expose identity, data, and authorization operations. Identity registration binds an identifier to a public key. Data registration associates an identifier with an owner, address, signature, and profile. Authorization operations maintain requests and owner decisions.

## Fabric contracts

Identity management, identifier management, and authorization management form the contract layer. Peers simulate contract execution, the client collects matching endorsements that satisfy the policy, and the ordering service orders transactions. Peers validate transactions and apply valid updates to the ledger.

The API's Fabric client handles transaction submission and queries for application-facing operations.

## Queries and DNS publication

Identity lookup returns a registered public key. The synchronization service is designed to periodically read committed identifier and authorization records, encode them as DNS resource records, and generate zone files for CoreDNS.

Clients retain private keys, Fabric stores management state, CoreDNS serves identifier records, and external data services store the original data.

## Completion states

Ledger commit indicates that a management record has been recorded. DNS visibility indicates that the corresponding record can be resolved. These states identify the outcome of an operation and its publication progress.

See the [running guide](runbook.md) for currently executable functionality.
