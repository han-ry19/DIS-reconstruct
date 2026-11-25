# Ledger and DNS implementation

This document describes the reconstruction’s ledger layout and DNS publication path. The published papers define the architecture and management workflows; implementation choices are described below.

## Ledger and management

The reconstruction stores data objects and authorization records separately. Each authorization is keyed by data identifier and recipient; its value retains the issuer, status, permissions, and expiration. Requests and decisions update the same authorization record. Auxiliary key families provide identifier mappings, owner and recipient associations, transaction information, and registration metadata.

The independent data/recipient key represents the authorization name AppID.DataID.data from *Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications*. Algorithms 5 and 8 address its TXT record using CreateCompositeKey(authorization, AppID.DataID.data, TXT). This implementation stores the logical record as an object under (data, recipient) and projects it to that DNS name. The paper defines the public record and workflows; the physical object layout is this reconstruction's implementation choice.

The signed API implements identity.register, data.register, data.update, authorization.request, authorization.decide, and authorization.update. Data and authorization updates follow the paper algorithms documented in [management operations](management-updates.md).

The reconstruction uses a versioned Ed25519 request envelope, actor-scoped retry receipts, and its current business-object schema. Fresh registrations write their own object and receipt records. GetSnapshot returns current identities, data objects, and authorization records. Request receipts stay on the ledger and are fetched individually for exact retries; snapshots do not read or return receipt history. Snapshot sequence is zero because publication sequence belongs to the event consumer.

## Event publication and DNS

Successful management operations emit ZoneUpdate events. When the persistent Redis projection is absent, the publisher captures the channel height and replays valid committed DIS events through the preceding block. It constructs the business state in memory and publishes it atomically before following later events. The publisher applies these events to Redis zone hashes. Each field is a relative DNS name; its JSON value describes the record types, with @ representing the zone apex.

The reconstructed publisher adds atomic updates of DNS fields, cached objects, and a block/transaction checkpoint. It resumes from the saved checkpoint after reconnecting. Empty-cache recovery uses the bounded block stream, while ordinary restarts and reconnection use the persistent checkpoint. The consumer retains current business objects and a publication count, without a per-request history set. Cold replay requires access to blocks from channel creation and takes work proportional to that history. These mechanisms are implemented in cmd/dis-sync and internal/dns.

CoreDNS is built with plugin/disredis. Ordinary queries use HGET; AXFR uses HGETALL and splits large transfers into multiple DNS messages. The uid and data zones expose A, NS, SOA, CERT, RP, URI, and TXT records. SOA serials track the publication sequence. Zone-file export is available for inspection.

## Deployment

[config.yaml](../config.yaml) selects organization counts, peer replicas, Raft orderers, endorsement, and machine placement. The API, publisher, Redis, and CoreDNS run on the controller.

This release uses ledger schema 4: independent authorizations, indexed queries, and event-position recovery form one storage path. Start it in a fresh runtime directory. Existing runtime directories retain their data and are rejected by the launcher if their ledger schema differs. This repository provides no ledger migration command.

## Publication path and measurement

The reconstruction uses committed events to update Redis and serves queries through the Redis-backed CoreDNS plugin. *Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications* describes periodic retrieval by its CoreDNS plugin in Section 8 and Algorithm 9.

Report the current implementation as the event-driven Redis path. Measure valid Fabric commit, commit-to-authoritative-DNS visibility, and authoritative query latency separately. Query throughput measures the serving path after publication; it does not measure synchronization delay. This reconstruction uses event-driven incremental publication. In Algorithm 9 (PDF page 21), a periodic check first detects changes, then traverses all current records of the affected identifier type to regenerate and transmit its record list. Section 8.1 (PDF page 30) places periodic retrieval in a CoreDNS plugin. The event reconnect interval controls reconnection only.

## Authorization concurrency

Different recipients have different authorization keys. Management operations read the target authorization directly; a recipient move also reads its destination to detect collisions. Each operation reads the data object to validate current ownership, but authorization changes do not rewrite that object. Authorization transaction information is stored per data/recipient, avoiding a shared per-data write.

Different-recipient approvals and updates can therefore commit after endorsement against the same state. Concurrent changes to one authorization, moves to the same recipient, and an ownership transfer that invalidates a pending authorization operation retain MVCC protection. A data-field update can also invalidate an authorization operation that read the previous data-object version.

Authorization benchmarks should distinguish distributed data objects, multiple recipients of one object, and repeated updates to one authorization. Report offered load, valid commits, MVCC invalidations, other failures, and retry policy separately.
