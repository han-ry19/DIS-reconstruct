# CoreDNS publication

The synchronization service reads committed Fabric state, writes the uid and data zones, and lets CoreDNS reload them. A complete TCP AXFR check compares both served zones with the ledger projection.

## Record mapping

| Object | DNS records |
|---|---|
| Identity | CERT public-key payload; TXT signature scheme |
| Data | RP owner; URI address; TXT data signature |
| Data profile | TXT at _profile.<data-identifier> |
| Authorization | TXT JSON at <application-label>.<data-identifier> |

The mapping follows Dominate's record types and authorization naming. The CERT payload uses the OID certificate type (254): the Ed25519 OID followed by the raw public key, with a length prefix for the OID. This is the reconstruction's key encoding. Algorithm metadata is Ed25519. Authorization JSON preserves the owner, recipient, status, permissions, and timestamps. TTL is 3600 seconds.

## Start the connected services

After starting the Fabric network and building the executables:

```bash
python3 scripts/services.py up
bin/dis-demo -url http://127.0.0.1:28080
bin/dis-sync -fabric-config runtime/fabric/gateway.json -out runtime/dns -once -verify 127.0.0.1:28530
```

Verification waits up to 15 seconds for CoreDNS to load the target zones. A successful publication log after -verify confirms that the full authoritative contents match the target snapshot. Without -verify, publication confirms that the zone files were written.

The service manager binds the API to loopback port 28080 and DNS to loopback port 28530 by default. Select --api-port and --dns-port on initial setup to use other ports.

```bash
python3 scripts/services.py status
python3 scripts/services.py down
```

Stopping services preserves published files. Restarting refreshes them from the Fabric ledger. The publisher rejects a snapshot older than the recorded published sequence, or a different snapshot at the same sequence. Each zone file is replaced atomically; the two zones converge through CoreDNS reload.

## Query records

With dig installed, use an identifier printed by the demo:

```bash
dig @127.0.0.1 -p 28530 DATA_IDENTIFIER URI
dig @127.0.0.1 -p 28530 DATA_IDENTIFIER RP
dig @127.0.0.1 -p 28530 APPLICATION_LABEL.DATA_IDENTIFIER TXT
```

Replace the uppercase placeholders with actual names. AXFR is enabled for loopback clients to support complete verification.
