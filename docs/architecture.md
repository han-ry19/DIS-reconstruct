# Architecture

This project reconstructs the DNS identifier registration, management, authorization, and resolution components described in Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership and Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications, following the papers and their source code.

## Management

The API accepts signed identity, data, authorization request, and owner-decision operations. Fabric peers execute the transitions under the chaincode endorsement policy. The API waits for a valid commit before reporting a successful update.

Composite ledger keys separate identities, data objects, pending authorizations, and retry receipts. Decisions update the confirmed authorization map embedded in a data object. Transaction IDs identify committed results, while actor-scoped receipts support exact retries and event deduplication. The DNS serial counts committed business transitions.

## Publication and resolution

A full snapshot initializes the Redis projection. Subsequent committed chaincode events update the affected DNS and cached object fields. Redis publishes those fields and the event checkpoint in one transaction. The publisher resumes from that checkpoint after a disconnect and loads a snapshot if it detects a sequence gap in the sequence-based ledger format. Fresh ledgers deduplicate committed events using their actor/request keys.

CoreDNS runs the disredis plugin and reads the requested zone and relative label from Redis. Zone-file exports are available for inspection and development. Normal Fabric publication is event-driven.

## Completion

A valid Fabric commit completes the ledger operation. A matching authoritative DNS answer completes publication. The acceptance runner checks the event-driven projection and both full DNS zones without triggering publication itself.

The [original implementation mapping](original-implementation.md) records the source revisions and component design used for the registration, authorization, and resolution workflow.
