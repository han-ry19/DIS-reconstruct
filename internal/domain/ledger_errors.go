package domain

import "errors"

// ErrLedgerConflict identifies a validated transaction that may be resubmitted.
var ErrLedgerConflict = errors.New("ledger conflict")
