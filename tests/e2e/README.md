# End-to-end verification

The connected scenario registers a user and application, registers data, requests access, records the owner's approval, and resolves the result through Redis-backed CoreDNS.

Run scripts/acceptance.py --restart to check four peers, full DNS records, exact replay, and component recovery. Run scripts/acceptance-events.py to prove event catch-up after offline commits without forcing a full publication. Results are saved under runtime.
