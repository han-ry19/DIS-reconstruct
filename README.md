# DIS-reconstruct

**Reconstructing the identifier registration, management, and resolution components of DIS described in DIA and Dominate.**

DIS connects users, applications, and data services through registered identities, resolvable data identifiers, and owner-controlled access authorization. This project translates the architecture and business workflows in DIA and Dominate into an implementation built on Hyperledger Fabric and CoreDNS.

## System design

The reconstruction covers the following capabilities:

| Capability | Purpose |
|---|---|
| Identity registration and lookup | Bind a user or application identity to its public key |
| Data registration | Bind a data identifier to its owner, address, signature, and profile |
| Data resolution | Retrieve the attributes associated with a data identifier |
| Authorization requests | Record an application's requested permissions and expiration time |
| Owner decisions | Approve or reject a pending request |
| Authorization resolution | Retrieve authorization status, permissions, and expiration |
| DNS publication | Publish committed identifier and authorization records through CoreDNS |

Users and applications submit requests through the DIS API. Fabric contracts validate and record management operations. A synchronization service projects committed records into DNS zones served by CoreDNS. Original data remains in external data services.

## Getting started

Follow the [running guide](docs/runbook.md) to run the signed registration and authorization workflow. The development API persists business state and request receipts across restarts. A Fabric shim adapter executes the same business rules; network deployment and DNS publication are the next integration steps.

## Documentation

- [Architecture](docs/architecture.md)
- [Mapping to DIA and Dominate](docs/dia-mapping.md)
- [Implementation decisions](docs/decisions.md)
- [Running guide](docs/runbook.md)
