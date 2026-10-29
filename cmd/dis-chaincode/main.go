package main

import (
	"github.com/han-ry19/DIS-reconstruct/chaincode/dis"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"log"
	"os"
)

func main() {
	var err error
	if address := os.Getenv("CHAINCODE_SERVER_ADDRESS"); address != "" {
		server := &shim.ChaincodeServer{CCID: os.Getenv("CHAINCODE_ID"), Address: address, CC: &dis.Contract{}, TLSProps: shim.TLSProperties{Disabled: true}}
		err = server.Start()
	} else {
		err = shim.Start(&dis.Contract{})
	}
	if err != nil {
		log.Fatal(err)
	}
}
