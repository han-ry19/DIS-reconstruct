# Mapping to DIA and Dominate

DIS-reconstruct implements the identity, data identifier, and authorization management path described in DIA and Dominate, together with DNS publication and resolution.

## Component mapping

| Paper component or capability | Reconstruction component | Business purpose |
|---|---|---|
| Identity Management | Identity contract and public-key lookup | Register identities and support requester authentication |
| Identifier Management | Data registration and attribute lookup | Bind data to a resolvable identifier |
| Authorization Management | Access requests and owner decisions | Maintain pending, approved, and rejected records |
| Blockchain Platform | Fabric network and contracts | Execute and record management operations |
| Identifier System | Synchronization service and CoreDNS | Resolve identifier and authorization records |
| Dominate's DIS-hub interface | DIS API | Provide an application-facing entry point |

## Business semantics

An application creates a pending authorization request. The data owner approves or rejects it. Fabric endorsement and commit validate and record those operations according to the contracts.

Data registration binds an identifier to its owner, address, data signature, and profile. Applications resolve data attributes and use the authorization workflow to request access.

## Design references

The DIA mapping follows Section III's components and interaction workflow, Algorithms 1–3, and Section IV's implementation. The Dominate mapping follows its system design and Section 8's implementation and evaluation.

The reconstruction focuses on identifier registration, management, and resolution. Identity lookup and data addresses provide the interfaces to identity and external data services; data discovery is an adjacent service in the broader architecture.

The original service, chaincode, and Redis-backed CoreDNS path are documented in [original implementation mapping](original-implementation.md).

Engineering choices are recorded in [implementation decisions](decisions.md). Executable components are described in the [running guide](runbook.md).
