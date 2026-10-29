# DIA/Dominate mapping

We reconstruct the identifier-management path described in DIA and extended by Dominate. This is not a claim of source-level recovery.

Paper concept -> component -> first milestone:

- Identity Management -> identity operations and API -> register and resolve public key.
- Identifier Management -> data identifier operations -> register and resolve owner, address, signature, and profile.
- Authorization Management -> pending request and owner decision -> request, approve/reject, resolve.
- Blockchain platform -> replaceable governance backend -> Fabric baseline first.
- Identifier System -> sync service and CoreDNS -> project committed state to DNS.
- DIS-hub -> dis-api -> HTTP entry point.
- PDS and ERDAP -> external or future boundary -> out of scope initially.

Owner authorization and organization endorsement are separate. The owner expresses the business decision; participating organizations verify that the request and state transition satisfy the shared contract.
