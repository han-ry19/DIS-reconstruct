package dis

import (
	"bytes"
	"encoding/json"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"io"
)

const stateKey = "dis.state.v1"

// Contract connects deterministic business transitions to the Fabric ledger.
// The initial implementation stores a complete state document per transaction.
type Contract struct{}

func (c *Contract) Init(stub shim.ChaincodeStubInterface) pb.Response {
	_, args := stub.GetFunctionAndParameters()
	if len(args) != 1 || len(args[0]) == 0 {
		return shim.Error("Init requires a domain")
	}
	existing, e := stub.GetState(stateKey)
	if e != nil {
		return shim.Error(e.Error())
	}
	if existing != nil {
		return shim.Error("already initialized")
	}
	state := domain.NewState(args[0])
	b, e := json.Marshal(state)
	if e != nil {
		return shim.Error(e.Error())
	}
	if e = stub.PutState(stateKey, b); e != nil {
		return shim.Error(e.Error())
	}
	return shim.Success(nil)
}
func (c *Contract) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	b, e := stub.GetState(stateKey)
	if e != nil {
		return shim.Error(e.Error())
	}
	if b == nil {
		return shim.Error("not initialized")
	}
	var state domain.State
	if e = json.Unmarshal(b, &state); e != nil {
		return shim.Error(e.Error())
	}
	switch function {
	case "Submit":
		if len(args) != 1 {
			return shim.Error("Submit requires signed request JSON")
		}
		var request domain.Request
		decoder := json.NewDecoder(bytes.NewBufferString(args[0]))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&request); e != nil {
			return shim.Error("invalid request JSON")
		}
		if e = decoder.Decode(new(any)); e != io.EOF {
			return shim.Error("invalid trailing JSON")
		}
		timestamp, e := stub.GetTxTimestamp()
		if e != nil {
			return shim.Error(e.Error())
		}
		previous := state.Sequence
		result, e := domain.Apply(&state, request, timestamp.Seconds)
		if e != nil {
			return shim.Error(e.Error())
		}
		if previous != state.Sequence {
			encoded, e := json.Marshal(state)
			if e != nil {
				return shim.Error(e.Error())
			}
			if e = stub.PutState(stateKey, encoded); e != nil {
				return shim.Error(e.Error())
			}
		}
		return success(result)
	case "GetSnapshot":
		if len(args) != 0 {
			return shim.Error("GetSnapshot takes no arguments")
		}
		return shim.Success(b)
	case "GetIdentity":
		if len(args) != 1 {
			return shim.Error("GetIdentity requires an identifier")
		}
		v, ok := state.Identities[args[0]]
		if !ok {
			return shim.Error("not found")
		}
		return success(v)
	case "GetData":
		if len(args) != 1 {
			return shim.Error("GetData requires an identifier")
		}
		v, ok := state.Data[args[0]]
		if !ok {
			return shim.Error("not found")
		}
		return success(v)
	case "GetAuthorization":
		if len(args) != 3 {
			return shim.Error("GetAuthorization requires data, owner, recipient")
		}
		v, ok := state.Authorizations[domain.AuthorizationKey(args[0], args[1], args[2])]
		if !ok {
			return shim.Error("not found")
		}
		return success(v)
	default:
		return shim.Error("unknown function")
	}
}
func success(value any) pb.Response {
	b, e := json.Marshal(value)
	if e != nil {
		return shim.Error(e.Error())
	}
	return shim.Success(b)
}
