# Original implementation and reconstruction

This project reconstructs the DNS identifier registration, management, authorization, and resolution components described in Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications, following the papers and their source code.

## Source references

| Repository | Inspected revision | Evidence |
|---|---|---|
| fuxi-inc/dip-dis-service | ad0b20b, 2024-02-28 | Fabric startup, signed API requests, transaction submission, startup zone loading, and event callbacks |
| fuxi-inc/dip-dis-service | a38fa32, 2024-07-10 | Paged full-zone loading into a temporary Redis key and replacement of the public zone |
| fuxi-inc/dip-dis-chaincode | 63c85ef, 2024-03-22 | Contract methods, composite keys, embedded confirmed authorizations, and DNS update events |
| fuxi-inc/dip-dis-chaincode | e795f6d, 2024-07-03, branch v2.3.1-zwy-fullupdatepartition | GetZoneRedisPagination |
| fuxi-inc/dis-coredns | df7d651, 2024-04-29, plugin/redis | Redis hash and JSON record representation, per-query HGET, and DNS response construction |

## Recovered mechanisms

- Fabric composite keys separate identities, data objects, pending authorization requests, and request receipts.
- Confirmed and rejected authorization records are embedded in their data object. A decision reads and rewrites that object, preserving the original implementation's object-level update dependency.
- Successful management transitions emit a ZoneUpdate event. The publisher receives committed events, applies business deltas, and changes the corresponding Redis DNS fields.
- Redis stores each authoritative zone as a hash: the key is the zone name, the field is the relative DNS label, and the value is JSON describing its record types. The apex uses @.
- CoreDNS 1.12.0 is built with the disredis plugin, which performs HGET for ordinary queries and HGETALL for complete AXFR verification.
- Startup performs a full committed-state load. Redis record updates and the event checkpoint are committed together. Reconnection resumes after the saved block and transaction; a sequence gap triggers full recovery.

## Component design

The signed API exposes identity.register, data.register, authorization.request, and authorization.decide. Ed25519 signatures authenticate requests, and authorization records progress through pending, approved, or rejected states.

Management transactions update object records, request receipts, and a shared metadata sequence. The shared sequence establishes the order consumed by the publisher and creates a common write dependency between management transactions.

The publisher holds the projection in memory, computes a complete-state digest, and writes changed object and DNS fields to Redis. GetSnapshot provides full recovery, and -once exports zone files.

The CoreDNS plugin serves the uid and data zones with A, NS, SOA, CERT, RP, URI, and TXT records. The SOA serial follows the business sequence, and large AXFR responses span multiple DNS messages.

The supplied deployment runs four peer organizations with a three-organization endorsement threshold and four Raft orderers.

## In-place migration

For an existing v1 deployment, stop the managed services and network, build the new commands, and run scripts/upgrade-ledger.py. The script starts the existing network, advances the chaincode definition sequence, and submits MigrateState. Migration creates object and receipt keys, removes the legacy state document, and preserves identifiers and historical results. The migration is idempotent.

The upgrade script saves the previous runtime/dns/status.json under runtime/reconstruction-backup before the first publication with the new digest format. The new publication digest excludes transaction receipts. Existing runtime records and certificates remain on the host. Fresh deployments start directly with the object layout.
