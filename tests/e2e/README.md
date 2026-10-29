# End-to-end verification

The acceptance scenario follows the DIA/Dominate identifier-management workflow:

Register user and application identities → register data → resolve its attributes → request access → record the owner's approval → resolve the approved authorization.

Checks will cover public keys, data attributes, authorization status, and DNS responses. Additional scenarios will exercise rejected requests, unauthorized operations, repeated submissions, and service restarts.

This workflow defines the reconstruction's business acceptance criteria.
