# Historical sources and reconstructed components

The paper algorithms guide identity, data, and authorization management. The inspected historical repositories supply the object-ledger organization and Redis-backed DNS implementation used in this reconstruction. Their correspondence to the revisions used in the published experiments has not been established. The revision references below identify the inspected implementation sources; they do not identify the papers’ experimental artifacts.

## Inspected sources

| Repository and revision | Source mechanisms |
|---|---|
| [fuxi-inc/dip-dis-service, ad0b20b](https://github.com/fuxi-inc/dip-dis-service/tree/ad0b20b), February 28, 2024 | Fabric startup, signed API requests, transaction submission, full-zone loading, event callbacks |
| [fuxi-inc/dip-dis-service, a38fa32](https://github.com/fuxi-inc/dip-dis-service/tree/a38fa32), July 10, 2024 | Paged loading into a temporary Redis key and replacement of the published zone |
| [fuxi-inc/dip-dis-chaincode, 63c85ef](https://github.com/fuxi-inc/dip-dis-chaincode/tree/63c85ef), March 22, 2024 | Contracts, composite keys, embedded decided authorizations, DNS update events |
| [fuxi-inc/dip-dis-chaincode, e795f6d](https://github.com/fuxi-inc/dip-dis-chaincode/tree/e795f6d), July 3, 2024 | GetZoneRedisPagination on branch v2.3.1-zwy-fullupdatepartition |
| [fuxi-inc/dis-coredns, df7d651](https://github.com/fuxi-inc/dis-coredns/tree/df7d651), April 29, 2024 | Redis zone hashes, JSON record values, per-query HGET, DNS answers |

## Ledger and management

The reconstruction retains the historical separation of data objects and pending authorization requests. Decided authorizations are embedded in the data object, so an owner decision reads and updates that object. Auxiliary key families provide identifier mappings, owner and recipient associations, transaction information, and registration metadata.

The signed API implements identity.register, data.register, data.update, authorization.request, authorization.decide, and authorization.update. Data and authorization updates follow the paper algorithms documented in [management operations](management-updates.md).

The reconstruction uses a versioned Ed25519 request envelope, actor-scoped retry receipts, and its current business-object schema. Fresh registrations write their own object and receipt records. Snapshots derive the publication sequence from committed receipts.

## Event publication and DNS

Following the historical implementation, startup loads the committed state and successful management operations emit ZoneUpdate events. The publisher applies these events to Redis zone hashes. Each field is a relative DNS name; its JSON value describes the record types, with @ representing the zone apex.

The reconstructed publisher adds atomic updates of DNS fields, cached objects, and a block/transaction checkpoint. It resumes from the saved checkpoint after reconnecting. Full snapshots support initialization and recovery. These mechanisms are implemented in cmd/dis-sync and internal/dns.

CoreDNS is built with plugin/disredis. Ordinary queries use HGET; AXFR uses HGETALL and splits large transfers into multiple DNS messages. The uid and data zones expose A, NS, SOA, CERT, RP, URI, and TXT records. SOA serials track the publication sequence. Zone-file export is available for inspection.

## Deployment and migration

[config.yaml](../config.yaml) selects organization counts, peer replicas, Raft orderers, endorsement, and machine placement. The API, publisher, Redis, and CoreDNS run on the controller.

For an earlier single-document deployment, stop its services and network, build the current commands, select its DIS_RUNTIME, and run scripts/upgrade-ledger.py. The script advances the chaincode definition and invokes MigrateState to preserve identifiers and receipts while converting the ledger. Migration is idempotent. The script saves the prior dns/status.json under reconstruction-backup before publication with the current digest format. Fresh deployments start with the object layout.

## Source locations

The following locations refer to dip-dis-chaincode revision 63c85ef11515e9183e7e2bc4b01fb34f1e390d00. Line numbers identify the inspected revision, rather than the moving default branch.

| File and lines | Mechanism visible in the source |
|---|---|
| contract/common.go, lines 19–26 | save constructs a Fabric composite key and calls PutState |
| contract/save_dataobject.go, lines 75–109; 134–165 | Registration builds a data object with an authorization map, stores its JSON, and writes registration metadata, association indexes, and transaction information |
| contract/save_confirmed_auth.go, lines 146–163 | Confirmation updates DataObject.Authorization, rewrites the data object, and maintains the recipient association index |
| contract/nameserver_get_zone.go, lines 235–283 | getZoneRedis scans DataObject records in a zone and converts each object into DNS records for Redis |

Pending requests are stored separately in contract/save_unconfirmed_auth.go, lines 131–133. For the service-side full-load path, dip-dis-service revision a38fa32, service/dis-q/zoneUpdatePagination.go, lines 76–132 fetches pages and writes a temporary Redis hash; lines 44–63 replace the published hash with RENAME. This is the paged initialization/reload path, not evidence of the incremental event mechanism by itself. In dis-coredns revision df7d651, plugin/redis/redis.go, line 425 reads a name using HGET.

These references document inspected sources even when repository access requires authorization. No license file was found in the inspected chaincode checkout; source excerpts are not redistributed here.

## Publication path and measurement

The reconstruction uses committed events to update Redis and serves queries through the Redis-backed CoreDNS plugin. The second paper describes periodic retrieval by its CoreDNS plugin in Section 8 and Algorithm 9. The synchronization path used for the published measurements has not been established from the inspected revisions.

Report the current implementation as the event-driven Redis path. Measure valid Fabric commit, commit-to-authoritative-DNS visibility, and authoritative query latency separately. Query throughput measures the serving path after publication; it does not measure synchronization delay. A periodic-retrieval variant would need its own implementation and explicit polling settings before it could be compared; changing the event reconnect interval does not reproduce Algorithm 9.

## Shared-object concurrency

Confirmed authorizations are embedded in their data object. Decisions and authorization updates for the same data object therefore read and write a shared Fabric key, even when their recipients differ. Transactions endorsed against the same version of that key compete at validation: after one valid update, another with the stale read version fails MVCC validation. Requests that execute against successive committed versions can both succeed.

Authorization benchmarks should distinguish operations distributed across data objects from operations concentrated on one object. Report offered load, valid commits, MVCC invalidations, other failures, and retry policy separately. This contention follows the retained object layout and should be interpreted alongside the workload's concentration.
