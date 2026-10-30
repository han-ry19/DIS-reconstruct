# Original implementation and reconstruction

The reconstruction uses the original DIS service, chaincode, and CoreDNS repositories to recover the registration, authorization, and resolution path on a maintained build environment.

## Source references

| Repository | Inspected revision | Evidence |
|---|---|---|
| fuxi-inc/dip-dis-service | ad0b20b, 2024-02-28 | Fabric startup, signed API requests, transaction submission, startup zone loading, and event callbacks |
| fuxi-inc/dip-dis-service | a38fa32, 2024-07-10 | Paged full-zone loading into a temporary Redis key and replacement of the public zone |
| fuxi-inc/dip-dis-chaincode | 63c85ef, 2024-03-22 | Contract methods, composite keys, embedded confirmed authorizations, and DNS update events |
| fuxi-inc/dip-dis-chaincode | e795f6d, 2024-07-03, branch v2.3.1-zwy-fullupdatepartition | GetZoneRedisPagination |
| fuxi-inc/dis-coredns | df7d651, 2024-04-29, plugin/redis | Redis hash and JSON record representation, per-query HGET, and DNS response construction |

The upstream repositories are inspected separately. This repository implements the recovered architecture through its existing typed interfaces; it does not embed the upstream repositories or import their operational configuration and credentials.

## Recovered mechanisms

- Fabric composite keys separate identities, data objects, pending authorization requests, and request receipts.
- Confirmed and rejected authorization records are embedded in their data object. A decision reads and rewrites that object, preserving the original implementation's object-level update dependency.
- Successful management transitions emit a ZoneUpdate event. The publisher receives committed events, applies business deltas, and changes the corresponding Redis DNS fields.
- Redis stores each authoritative zone as a hash: the key is the zone name, the field is the relative DNS label, and the value is JSON describing its record types. The apex uses @.
- CoreDNS 1.12.0 is built with the disredis plugin, which performs HGET for ordinary queries and HGETALL for complete AXFR verification.
- Startup performs a full committed-state load. Redis record updates and the event checkpoint are committed together. Reconnection resumes after the saved block and transaction; a sequence gap triggers full recovery.

## Current adaptation boundaries

The current signed API preserves the existing identity.register, data.register, authorization.request, and authorization.decide operations. Its Ed25519 request encoding and pending/approved/rejected representation are reconstruction interfaces, not wire-compatible copies of the original DIP API. The original chaincode's DOI alias maps, extended ownership/delegation operations, and paginated full-zone API have not been ported.

Business sequence and receipt fields remain compatible with the initial reconstruction. A shared metadata sequence is still updated by management transactions; concurrency measurements must account for that dependency. The state layout is object-based, but this revision is not an implementation of independent-object concurrent commits.

The publisher retains the projection in memory and computes its digest over the complete state. It writes changed DNS/object fields to Redis; it does not query Fabric or rewrite zone files for every event. Full-zone export remains available through -once. Full recovery currently uses GetSnapshot rather than the original paginated query.

The plugin preserves the original JSON fields and adds a stable SOA serial for full verification. Its supported projection types are A, NS, SOA, CERT, RP, URI, and TXT. Zone discovery is fixed to uid and data. It is an adaptation on CoreDNS 1.12.0, not a build of the complete old fork.

The four-organization, three-endorsement Fabric policy is this reconstruction's deployment configuration. The inspected upstream lifecycle scripts do not specify an explicit chaincode endorsement policy; the original effective policy requires the matching channel configuration.

## In-place migration

For an existing v1 deployment, stop the managed services and network, build the new commands, and run scripts/upgrade-ledger.py. The script starts the existing network, advances the chaincode definition sequence, and submits MigrateState. Migration creates object and receipt keys, removes the legacy state document, and preserves identifiers and historical results. The migration is idempotent.

Save the previous runtime/dns/status.json before the first publication with the new digest format. The new publication digest excludes transaction receipts. Existing runtime records and certificates remain on the host. Fresh deployments start directly with the object layout.
