# Redis-backed CoreDNS

The connected deployment follows the original DIS publication architecture: committed Fabric events update Redis, and a CoreDNS plugin answers DNS queries from Redis records.

## Components

- cmd/dis-sync: full-state bootstrap, committed event consumption, Redis publication, and restart checkpoints.
- plugin/disredis: a CoreDNS 1.12.0 adapter for the original DIS per-label JSON format.
- cmd/dis-coredns: builds CoreDNS with the adapter, bind, and errors plugins.
- Redis 7.2.5: installed from a pinned, checksum-verified source archive by scripts/bootstrap.py.

The Redis zone keys are uid. and data.; fields are relative names, with @ for the zone apex. Each field contains JSON arrays for supported record types. A stable SOA serial follows the business sequence.

## Run and verify

After building all commands and starting Fabric:

```bash
python3 scripts/services.py up
bin/dis-demo -url http://127.0.0.1:28080
bin/dis-sync -fabric-config runtime/fabric/gateway.json -redis 127.0.0.1:28679 -check -verify 127.0.0.1:28530
python3 scripts/acceptance-events.py
```

The check command compares the Redis projection and full authoritative AXFR results with Fabric. It does not publish. The event acceptance stops the publisher, commits a workflow while it is offline, and verifies event catch-up after restart.

The service manager uses loopback API port 28080, DNS port 28530, and Redis port 28679. Initial ports are configurable. Redis AOF persistence and the saved event position support restart recovery. Public DNS records, object-cache changes, and checkpoint updates share a Redis transaction.

## Export zone files

```bash
bin/dis-sync -fabric-config runtime/fabric/gateway.json -out runtime/dns -once
```

This command exports a full snapshot for inspection. The connected CoreDNS process reads Redis, not these files.

See the [original implementation mapping](../../docs/original-implementation.md) for source revisions and adaptation details.
