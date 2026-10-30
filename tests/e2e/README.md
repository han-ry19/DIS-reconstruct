# End-to-end verification

The connected scenario registers a user and application, registers data, requests access, records the owner's approval, and resolves the result through Redis-backed CoreDNS.

Run scripts/acceptance.py --restart --all-gateways to check every configured peer, Gateway, replica endorsement, full DNS records, exact replay, and component recovery. Run scripts/acceptance-events.py to prove event catch-up after offline commits without forcing a full publication. Results are saved under DIS_RUNTIME, which defaults to runtime in the checkout. See the [experiment deployment guide](../../docs/experiment-deployment.md) for topology selection.
