# Mapping to Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications

DIS-reconstruct implements the identity, data identifier, and authorization management path described in Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications, together with DNS publication and resolution.

## Component mapping

| Paper component or capability | Reconstruction component | Business purpose |
|---|---|---|
| Identity Management | Identity contract and public-key lookup | Register identities and support requester authentication |
| Identifier Management | Data registration, attribute updates, and lookup | Bind data to a resolvable identifier |
| Authorization Management | Access requests, owner decisions, and field updates | Maintain pending, approved, and rejected records |
| Blockchain Platform | Fabric network and contracts | Execute and record management operations |
| Identifier System | Synchronization service and CoreDNS | Resolve identifier and authorization records |
| Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications's DIS-hub interface | DIS API | Provide an application-facing entry point |

## Business semantics

An application creates a pending authorization request. The data owner approves or rejects it. Fabric endorsement and commit validate and record those operations according to the contracts.

Data registration binds an identifier to its owner, address, data signature, and profile. Applications resolve data attributes and use the authorization workflow to request access.

## Design references

The Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership mapping follows Section III's components and interaction workflow, Algorithms 1–3, and Section IV's implementation. The Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications mapping follows its system design and Section 8's implementation and evaluation.

The reconstruction focuses on identifier registration, management, and resolution. Identity lookup and data addresses provide the interfaces to identity and external data services; data discovery is an adjacent service in the broader architecture.

The original service, chaincode, and Redis-backed CoreDNS path are documented in [original implementation mapping](original-implementation.md).

Engineering choices are recorded in [implementation decisions](decisions.md). Executable components are described in the [running guide](runbook.md).

## Original-text locations

Page numbers below are PDF viewer pages. For Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership, PDF page 9 is printed page 446. The other paper uses Page X of 39 headers.

| Capability | Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership | Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications |
|---|---|---|
| Identity | Page 8, Section IV: Orb/W3C DID | Pages 12–14, Section 5.1.1, Figure 6 and Algorithm 2: .uid, CERT and TXT |
| Data registration | Page 4, Algorithm 1 | Pages 15–16, Algorithm 3 |
| Authorization request | Page 4, Algorithm 2 and Listing 1 | Page 16, Algorithm 4 |
| Owner decision | Page 5, Algorithm 3 | Page 17, Algorithm 5 |
| DNS publication | Pages 6 and 8: periodic synchronization | Page 21, Section 5.2.1 and Algorithm 9; page 30 implementation |
| Fixed-size blockchain experiment | Page 9, Section IV-B and Figure 5 | Pages 31–32, Section 8.2 and Figure 12 |
| DNS experiment | Pages 9–10, Section IV-C and Figure 6 | Pages 32–33, Section 8.3 and Figure 13 |

Acceptance covers identity registration and lookup, data registration and resolution, authorization requests and owner decisions, and committed-state DNS publication. The executable reconstruction uses the DNS identity model and inspected historical object-based ledger and event/Redis publication path. See [implementation decisions](decisions.md) and [historical source references](original-implementation.md).

Data and authorization updates implement Section 5.1.5, Algorithms 7–8, PDF pages 18–20 of Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications. See [management operations](management-updates.md) for field semantics, tests, and historical auxiliary-key sources. Orb, data discovery, and complete external-data access are separate extensions. DiSAuth is a candidate for related work and does not define reconstruction functionality or experimental settings.
