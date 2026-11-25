# Redis-backed CoreDNS

Committed Fabric events update Redis; CoreDNS answers DNS queries through the disredis plugin.

| Component | Role |
|---|---|
| cmd/dis-sync | Checkpoint recovery, empty-cache history replay, incremental event publication |
| internal/dns | Record projection, Redis updates, and complete-zone verification |
| plugin/disredis | Redis lookup and DNS response construction |
| cmd/dis-coredns | CoreDNS with disredis, bind, and errors plugins |

## Start and verify

The unified launcher starts these components with Fabric:

~~~bash
./run.sh
bin/dis-demo -url http://127.0.0.1:38080
bin/dis-sync \
  -redis 127.0.0.1:38679 -check -verify 127.0.0.1:38530
~~~

These addresses belong to the default config.yaml. Change them for another deployment. The check command reads the persistent Redis projection and verifies both zones against it by TCP AXFR, without a Fabric connection. Use dis-audit for explicit comparison with committed history; see the [running guide](../../docs/runbook.md#audit-committed-history).

## Records and persistence

Redis stores zones in the uid. and data. hashes. Fields are relative names, with @ for the apex; values contain JSON record arrays. Identity records expose CERT/TXT, data records RP/URI/TXT, and authorization records TXT. SOA serials follow the publication sequence.

Ordinary queries use HGET; complete-zone transfer uses HGETALL. Redis AOF persistence retains published records. Record changes, cached business objects, and event checkpoints are written in the same Redis transaction. The publisher reconnects using the saved event position.

## Zone-transfer access

AXFR accepts loopback TCP clients by default. A manually managed Corefile can use transfer_allow inside disredis to list permitted IP addresses or CIDR ranges, for example:

~~~text
disredis {
  address 127.0.0.1:38679
  transfer_allow 127.0.0.1/32 ::1/128 192.0.2.0/24
}
~~~

Replace the documentation network with the intended replica network. transfer_allow none disables zone transfer. Keep loopback access enabled when running the supplied AXFR verification commands. The unified launcher generates its Corefile from the deployment settings; manage a separate Corefile when customizing plugin options.

## Zone-file export

~~~bash
bin/dis-sync -fabric-config runtime/configured/fabric/gateway.json \
  -out runtime/configured/dns -once
~~~

The export supports inspection. CoreDNS serves records from Redis. External data services use the published identity and authorization records to authenticate requesters and decide access.
