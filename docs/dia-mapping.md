# Source architecture and implementation mapping

The reconstruction follows [Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership](https://ieeexplore.ieee.org/document/10197025) and [Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications](https://link.springer.com/article/10.1007/s11280-025-01337-z).

The architecture is shown in Figure 1 of the first paper and Figure 4 of the second. The [README schematic](../README.md#system-design) redraws these figures and highlights the reconstructed path.

## Components

| Source component | Repository implementation | Role |
|---|---|---|
| Identity Management | Identity contract and public-key queries | Register user and application identities and authenticate signed requests |
| Data Management | Data registration, field updates, and queries | Bind identifiers to owners, addresses, signatures, and profiles |
| Authorization Management | Requests, owner decisions, and field updates | Maintain application permissions and expiration |
| Blockchain Platform | Fabric peers, Raft orderers, and contracts | Validate and record management operations |
| Update Identifier | Snapshot and committed-event publisher | Publish ledger records to Redis |
| Identifier System | Redis-backed CoreDNS | Resolve identity, data, and authorization records |
| DIS-hub interface in the second paper | DIS API | Provide application-facing management and query endpoints |

The first paper's implementation uses Orb/W3C DID for identity. The reconstruction uses the DNS identity model developed in the second paper. Registration metadata uses the first paper’s label, format, and description fields. These fields remain in the ledger and are excluded from DNS, following the second paper’s record projection. Metadata discovery belongs to the broader architecture; personal data storage and enforcement run in external data services.

## Original-text locations

Page numbers below are PDF viewer pages. In the first paper, PDF page 9 is printed page 446. The second paper uses “Page X of 39” headers.

| Capability | Data Interoperating Architecture (DIA): Decoupling Data and Applications to Give Back Your Data Ownership | Dominate data by yourself: a decentralized scheme for data interoperation when data is decoupled from applications |
|---|---|---|
| Architecture | Figure 1 | Figure 4 |
| Identity | Page 8, Section IV: Orb/W3C DID | Pages 12–14, Section 5.1.1, Figure 6 and Algorithm 2 |
| Data registration | Page 4, Algorithm 1 | Pages 15–16, Algorithm 3 |
| Authorization request | Page 4, Algorithm 2 and Listing 1 | Page 16, Algorithm 4 |
| Owner decision | Page 5, Algorithm 3 | Page 17, Algorithm 5 |
| Data and authorization updates | — | Pages 18–20, Section 5.1.5, Algorithms 7–8 |
| DNS publication | Pages 6 and 8 | Page 21, Section 5.2.1 and Algorithm 9; page 30 implementation |
| Blockchain experiment | Page 9, Section IV-B, Figure 5 | Pages 31–32, Section 8.2, Figure 12 |
| DNS experiment | Pages 9–10, Section IV-C, Figure 6 | Pages 32–33, Section 8.3, Figure 13 |

## Implementation sources

The paper algorithms define the business workflow. The reconstruction uses an object ledger and an event-driven Redis/CoreDNS path; see [ledger and DNS implementation](original-implementation.md). Request encoding, retries, and recovery are described in [implementation decisions](decisions.md). [Management operations](management-updates.md) defines the implemented updates, and [verification](verification.md) describes executable checks.
