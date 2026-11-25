# End-to-end verification

The connected workflow registers a user and application, registers data, requests access, records the owner's decision, and verifies the resulting ledger and DNS records.

~~~bash
./run.sh
export DIS_RUNTIME="$PWD/runtime/configured"
python3 scripts/acceptance.py --restart --all-gateways
python3 scripts/acceptance-events.py
~~~

Use a dedicated test deployment. The first command sequence checks every configured peer and Gateway, explicit replica endorsements, complete DNS zones, exact retries, and component recovery. The event test stops the publisher during transactions and checks checkpoint-based catch-up. Reports are saved under DIS_RUNTIME. See [verification](../../docs/verification.md).
