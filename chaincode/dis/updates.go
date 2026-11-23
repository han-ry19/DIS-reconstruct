package dis

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
)

func persistAuthorizationUpdate(stub shim.ChaincodeStubInterface, s domain.State, r domain.Request, d dataObject, result domain.Result) error {
	a := s.Authorizations[result.ObjectID]
	owner := r.Parameters.AuthorizationOwner
	if owner == "" {
		owner = d.Data.OwnerID
	}
	oldKey := domain.AuthorizationKey(a.DataID, owner, r.Parameters.Recipient)
	var pending domain.AuthorizationRecord
	isPending, err := read(stub, pendingKind, &pending, a.DataID, owner, r.Parameters.Recipient)
	if err != nil {
		return err
	}
	delete(d.Authorizations, oldKey)
	if isPending && a.Status == domain.AuthorizationPending {
		if err := remove(stub, pendingKind, a.DataID, owner, r.Parameters.Recipient); err != nil {
			return err
		}
		// A pending record may overlay a previously decided authorization.
		// Persist its removal as well, so renaming cannot resurrect the old grant.
		if err := put(stub, dataKind, d, a.DataID); err != nil {
			return err
		}
		return put(stub, pendingKind, a, a.DataID, a.OwnerID, a.Recipient)
	}
	d.Authorizations[result.ObjectID] = a
	if err := put(stub, dataKind, d, a.DataID); err != nil {
		return err
	}
	if isPending {
		return remove(stub, pendingKind, a.DataID, owner, r.Parameters.Recipient)
	}
	return nil
}
