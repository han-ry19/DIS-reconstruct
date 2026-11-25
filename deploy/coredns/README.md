# Redis-backed CoreDNS

Committed Fabric events update Redis; CoreDNS answers DNS queries through the disredis plugin.

| Component | Role |
|---|---|
| cmd/dis-sync | Initial snapshot, committed event consumption, publication, and recovery checkpoints |
| internal/dns | Record projection, Redis updates, and complete-zone verification |
| plugin/disredis | Redis lookup and DNS response construction |
| cmd/dis-coredns | CoreDNS with disredis, bind, and errors plugins |

## Start and verify

The unified launcher starts these components with Fabric:

~~~bash
./run.sh
bin/dis-demo -url http://127.0.0.1:38080
bin/dis-sync -fabric-config runtime/configured/fabric/gateway.json \
  -redis 127.0.0.1:38679 -check -verify 127.0.0.1:38530
~~~

These addresses belong to the default config.yaml. Change them for another deployment. The check command reads committed state, compares the Redis projection, and verifies both zones by TCP AXFR.

## Records and persistence

Redis stores zones in the uid. and data. hashes. Fields are relative names, with @ for the apex; values contain JSON record arrays. Identity records expose CERT/TXT, data records RP/URI/TXT, and authorization records TXT. SOA serials follow the publication sequence.

Ordinary queries use HGET; complete-zone transfer uses HGETALL. Redis AOF persistence retains published records. Record changes, cached business objects, and event checkpoints are written in the same Redis transaction. The publisher reconnects using the saved event position.

## Zone-file export

~~~bash
bin/dis-sync -fabric-config runtime/configured/fabric/gateway.json \
  -out runtime/configured/dns -once
~~~

The export supports inspection. CoreDNS serves records from Redis. External data services use the published identity and authorization records to authenticate requesters and decide access.
