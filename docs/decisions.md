# Implementation decisions

## Signed requests and deterministic execution

Ed25519 signatures bind the deployment domain, operation, actor, request ID, timestamp, and typed parameters. SigningBytes in internal/domain defines the encoding. The JavaScript implementation in benchmarks/caliper/signing.js uses the same field order, omitted values, and JSON escaping; cross-language tests compare the resulting bytes. Clients generate a random UUIDv4 before signing and use its lowercase canonical form followed by .data. The contract validates the version and variant bits, format, and uniqueness, so every endorsing peer validates the same input. Human-readable names belong in profile.label. Existing registered identifiers remain readable and manageable. The contract validates the supplied UUID, not the client’s source of randomness.

Request IDs are scoped to their actor. An exact retry returns the recorded result; changed content with the same ID is rejected. New requests must fall within five minutes of the execution timestamp. The Fabric adapter uses GetTxTimestamp; the file backend uses its execution clock.

The data owner decides application permissions. Fabric peers verify the signed operation and its business conditions, then endorse the execution result. Ordering and commit follow the configured Fabric policies.

## Ledger organization

Fabric stores identities, data objects, authorizations, and receipts under composite keys. Pending and decided authorizations use independent data/recipient records, retaining the issuer in their values. Identifier mappings, owner and recipient associations, transaction information, and registration metadata support management queries. The object layout is an implementation choice; the DNS projection defines the externally resolved records. Performance reports should identify this layout and its write sets.

Fresh ledgers identify successful transitions by transaction ID and actor-scoped receipts. Snapshots derive the publication sequence from committed receipts, keeping unrelated registrations' write sets independent. MigrateState converts earlier document and embedded-authorization layouts to independent authorizations and receipt-derived sequencing while preserving the published sequence.

The development backend stores an atomic JSON document protected by a single-writer lock.

## Authorization updates

A newly signed authorization request writes a pending record at the data/recipient key. Any existing record at that data/recipient name prevents another application request, including pending, approved, rejected, expired, and previous-owner records. A new request returns a conflict without changing the record. The current owner manages status, permissions, recipient, and expiration through authorization.update; an exact retry of the original signed request returns its saved result even after a later owner decision. An owner decision sets status and issuance time while retaining the requested permissions and expiration. Field updates follow Algorithms 7–8 of *Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications*. See [management operations](management-updates.md) for ownership transfer, recipient changes, and pending-record behavior.

## DNS publication

The publisher loads a committed snapshot, then consumes committed events. Redis transactions update DNS fields, cached business objects, and the recovery checkpoint together. Full-state digests are computed for snapshots and same-sequence verification; ordinary incremental publication writes the changed records and checkpoint without serializing the full state. CoreDNS reads the resulting zone hashes.

Identity records use CERT/TXT, data records RP/URI/TXT, and authorization records TXT. Ed25519 CERT type 254 carries an OID-length byte, the DER OID value for 1.3.101.112, and the raw key. Authorization TXT uses expired_at, matching the paper’s external record format; signed API requests and stored objects use expires_at. Profiles retain the first paper’s label, format, and description as registration metadata and are excluded from DNS. Complete AXFR verification compares authoritative records, including SOA serials, with the ledger projection.

## Completion and data consumption

A successful API write requires valid Fabric commit status. The result's committed_at field contains the transaction's execution timestamp, supplied by GetTxTimestamp in Fabric. Measure client-observed commit latency separately. DNS publication completes when the corresponding authoritative records are available.

The stored data signature is an owner-supplied attribute with validated encoding. External data services retain the payload and use the published identity, data, and authorization records for authentication and access control.
