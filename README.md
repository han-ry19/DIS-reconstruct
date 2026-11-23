# DIS-reconstruct

This project reconstructs the DNS identifier registration, management, authorization, and resolution components described in Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications, following the papers and their source code.

## System design

The reconstruction covers the following capabilities:

| Capability | Purpose |
|---|---|
| Identity registration and lookup | Bind a user or application identity to its public key |
| Data registration | Bind a data identifier to its owner, address, signature, and profile |
| Data updates | Change the address, data signature, or current owner |
| Data resolution | Retrieve the attributes associated with a data identifier |
| Authorization requests | Record an application's requested permissions and expiration time |
| Owner decisions | Approve or reject a pending request |
| Authorization updates | Change status, permissions, recipient, or expiration |
| Authorization resolution | Retrieve authorization status, permissions, and expiration |
| DNS publication | Publish committed identifier and authorization records through CoreDNS |

Deployment topology is configurable for experiments with different organization, peer, orderer, and machine counts. See the [experiment deployment guide](docs/experiment-deployment.md).

Users and applications submit requests through the DIS API. Fabric contracts validate and record management operations. A synchronization service applies committed chaincode events to Redis records served by a CoreDNS plugin. Original data remains in external data services.

## Configuration

Use [config.yaml](config.yaml) and the [configuration guide](config/README.md) to configure and launch a deployment with one file. Run `./run.sh plan`, then `./run.sh` to start with a configuration summary and readiness logs.

## Getting started

Follow the [running guide](docs/runbook.md) to start the Fabric network and CoreDNS services, run the signed registration and authorization workflow, and verify that the authoritative DNS zones match committed records.

## Documentation

- [Data and authorization management](docs/management-updates.md)
- [Architecture](docs/architecture.md)
- [Mapping to Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications](docs/dia-mapping.md)
- [Implementation decisions](docs/decisions.md)
- [Running guide](docs/runbook.md)
- [System verification](docs/verification.md)
- [Experiment deployment](docs/experiment-deployment.md)
- [Original implementation mapping](docs/original-implementation.md)
