# CoreDNS deployment

CoreDNS serves the resolution view of DIS management records. Data identifiers resolve to their registered attributes, and authorization identifiers resolve to the associated authorization records.

This directory is reserved for zone templates, record encodings, and server configuration. Together with the synchronization service, these files will connect committed ledger records to DNS responses.

Publication checks will compare actual DNS responses with the intended committed records. Configuration and startup commands will be documented in the [running guide](../../docs/runbook.md) when the publication service is connected.
