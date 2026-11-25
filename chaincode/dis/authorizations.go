package dis

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
)

const authorizationKind = "Authorization"
const authorizationTransactionKind = "AuthorizationTransactionInfo"

// One record owns each externally visible data/recipient authorization name.
// Owner remains a value so existing issuer checks and ownership transfers persist.
func loadAuthorization(stub shim.ChaincodeStubInterface, s *domain.State, data, recipient string) error {
	if recipient == "" {
		return nil
	}
	var a domain.AuthorizationRecord
	ok, err := read(stub, authorizationKind, &a, data, recipient)
	if err != nil {
		return err
	}
	if ok {
		s.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
	}
	return nil
}

func persistIndependentAuthorization(stub shim.ChaincodeStubInterface, s domain.State, r domain.Request, result domain.Result) error {
	a := s.Authorizations[result.ObjectID]
	if r.Operation == "authorization.update" && r.Parameters.Recipient != a.Recipient {
		if err := remove(stub, authorizationKind, a.DataID, r.Parameters.Recipient); err != nil {
			return err
		}
	}
	return put(stub, authorizationKind, a, a.DataID, a.Recipient)
}

// Upgrade during maintenance with management writes stopped. Fabric validates
// range reads atomically; a concurrent management write causes a retry, not loss.
func migrateAuthorizations(stub shim.ChaincodeStubInterface) error {
	m, err := getMetadata(stub)
	if err != nil || m.IndependentAuthorizations {
		return err
	}
	s, err := snapshot(stub, m)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(s.Authorizations))
	for k := range s.Authorizations {
		names = append(names, k)
	}
	sort.Strings(names)
	seen := map[string]bool{}
	for _, k := range names {
		a := s.Authorizations[k]
		name := a.DataID + "|" + a.Recipient
		if seen[name] {
			return fmt.Errorf("multiple issuers for authorization %s", name)
		}
		seen[name] = true
		if err := put(stub, authorizationKind, a, a.DataID, a.Recipient); err != nil {
			return err
		}
	}
	names = names[:0]
	for id := range s.Data {
		names = append(names, id)
	}
	sort.Strings(names)
	for _, id := range names {
		if err := put(stub, dataKind, dataObject{Data: s.Data[id]}, id); err != nil {
			return err
		}
	}
	var pendingKeys []string
	it, err := stub.GetStateByPartialCompositeKey(pendingKind, nil)
	if err != nil {
		return err
	}
	for it.HasNext() {
		v, e := it.Next()
		if e != nil {
			it.Close()
			return e
		}
		pendingKeys = append(pendingKeys, v.Key)
	}
	it.Close()
	for _, k := range pendingKeys {
		if err := stub.DelState(k); err != nil {
			return err
		}
	}
	if !m.ReceiptSequence {
		receipts, err := stub.GetStateByPartialCompositeKey(receiptKind, nil)
		if err != nil {
			return err
		}
		var keys []string
		var values []domain.Receipt
		for receipts.HasNext() {
			entry, err := receipts.Next()
			if err != nil {
				receipts.Close()
				return err
			}
			_, parts, err := stub.SplitCompositeKey(entry.Key)
			if err != nil || len(parts) != 2 {
				receipts.Close()
				return fmt.Errorf("invalid receipt key")
			}
			var receipt domain.Receipt
			if err := json.Unmarshal(entry.Value, &receipt); err != nil {
				receipts.Close()
				return err
			}
			receipt.Actor = parts[0]
			receipt.Result.RequestID = parts[1]
			keys = append(keys, entry.Key)
			values = append(values, receipt)
		}
		receipts.Close()
		if uint64(len(keys)) > m.Sequence {
			return fmt.Errorf("receipt count exceeds sequence")
		}
		for i, k := range keys {
			value, err := json.Marshal(values[i])
			if err != nil {
				return err
			}
			if err := stub.PutState(k, value); err != nil {
				return err
			}
		}
		m.Sequence -= uint64(len(keys))
		m.ReceiptSequence = true
	}
	m.IndependentAuthorizations = true
	if err := putMetadata(stub, m); err != nil {
		return err
	}
	return stub.SetEvent(domain.FullUpdateEvent, []byte("{}"))
}

func authorizationTransaction(stub shim.ChaincodeStubInterface, a domain.AuthorizationRecord, created int64) error {
	ts, err := stub.GetTxTimestamp()
	if err != nil {
		return err
	}
	info := transactionInfo{Identifier: a.DataID, TransactionID: stub.GetTxID(), CreatedAt: created, UpdatedAt: ts.Seconds, UpdatedAtNanos: ts.Nanos}
	index, err := rawIndex(stub, doiToIndexKind, a.DataID)
	if err != nil {
		return err
	}
	info.Index = index
	return put(stub, authorizationTransactionKind, info, a.DataID, a.Recipient)
}

func latestAuthorizationTransaction(stub shim.ChaincodeStubInterface, id string, base transactionInfo) (transactionInfo, error) {
	it, err := stub.GetStateByPartialCompositeKey(authorizationTransactionKind, []string{id})
	if err != nil {
		return base, err
	}
	defer it.Close()
	for it.HasNext() {
		entry, err := it.Next()
		if err != nil {
			return base, err
		}
		var info transactionInfo
		if err := json.Unmarshal(entry.Value, &info); err != nil {
			return base, err
		}
		if info.UpdatedAt > base.UpdatedAt || info.UpdatedAt == base.UpdatedAt && (info.UpdatedAtNanos > base.UpdatedAtNanos || info.UpdatedAtNanos == base.UpdatedAtNanos && info.TransactionID > base.TransactionID) {
			base = info
		}
	}
	return base, nil
}
