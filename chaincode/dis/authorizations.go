package dis

import (
	"encoding/json"

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
