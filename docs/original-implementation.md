# Ledger and DNS implementation

This document describes the reconstruction’s ledger layout and DNS publication path. The published papers define the architecture and management workflows; implementation choices are described below.

## Ledger and management

The reconstruction stores data objects and authorization records separately. Each authorization is keyed by data identifier and recipient; its value retains the issuer, status, permissions, and expiration. Requests and decisions update the same authorization record. Auxiliary key families provide identifier mappings, owner and recipient associations, transaction information, and registration metadata.

The signed API implements identity.register, data.register, data.update, authorization.request, authorization.decide, and authorization.update. Data and authorization updates follow the paper algorithms documented in [management operations](management-updates.md).

The reconstruction uses a versioned Ed25519 request envelope, actor-scoped retry receipts, and its current business-object schema. Fresh registrations write their own object and receipt records. Snapshots derive the publication sequence from committed receipts.

## Event publication and DNS

At startup, the publisher loads the committed state and successful management operations emit ZoneUpdate events. The publisher applies these events to Redis zone hashes. Each field is a relative DNS name; its JSON value describes the record types, with @ representing the zone apex.

The reconstructed publisher adds atomic updates of DNS fields, cached objects, and a block/transaction checkpoint. It resumes from the saved checkpoint after reconnecting. Full snapshots support initialization and recovery. These mechanisms are implemented in cmd/dis-sync and internal/dns.

CoreDNS is built with plugin/disredis. Ordinary queries use HGET; AXFR uses HGETALL and splits large transfers into multiple DNS messages. The uid and data zones expose A, NS, SOA, CERT, RP, URI, and TXT records. SOA serials track the publication sequence. Zone-file export is available for inspection.

## Deployment and migration

[config.yaml](../config.yaml) selects organization counts, peer replicas, Raft orderers, endorsement, and machine placement. The API, publisher, Redis, and CoreDNS run on the controller.

For an earlier single-document or embedded-authorization deployment, stop its services and network, build the current commands, select its DIS_RUNTIME, and run scripts/upgrade-ledger.py. The script advances the chaincode definition and invokes MigrateState to preserve identifiers and receipts while converting the ledger. Migration is idempotent. The script saves the prior dns/status.json under reconstruction-backup before publication with the current digest format. Fresh deployments start with independent authorization records. Migration preserves the visible authorization state, including pending records that superseded an earlier decision, and retains request receipts. Run the upgrade during a maintenance window with management writes stopped. The current chaincode requires migration before accepting writes from an older ledger layout.

## Publication path and measurement

The reconstruction uses committed events to update Redis and serves queries through the Redis-backed CoreDNS plugin. *Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications* describes periodic retrieval by its CoreDNS plugin in Section 8 and Algorithm 9.

Report the current implementation as the event-driven Redis path. Measure valid Fabric commit, commit-to-authoritative-DNS visibility, and authoritative query latency separately. Query throughput measures the serving path after publication; it does not measure synchronization delay. A periodic-retrieval variant would need its own implementation and explicit polling settings before comparison; changing the event reconnect interval does not reproduce Algorithm 9.

## Authorization concurrency

Different recipients have different authorization keys. Management operations read the target authorization directly; a recipient move also reads its destination to detect collisions. Each operation reads the data object to validate current ownership, but authorization changes do not rewrite that object. Authorization transaction information is stored per data/recipient, avoiding a shared per-data write.

Different-recipient approvals and updates can therefore commit after endorsement against the same state. Concurrent changes to one authorization, moves to the same recipient, and an ownership transfer that invalidates a pending authorization operation retain MVCC protection. A data-field update can also invalidate an authorization operation that read the previous data-object version.

Authorization benchmarks should distinguish distributed data objects, multiple recipients of one object, and repeated updates to one authorization. Report offered load, valid commits, MVCC invalidations, other failures, and retry policy separately.
