package main

import (
	"github.com/han-ry19/DIS-reconstruct/chaincode/dis"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"log"
)

func main() {
	if err := shim.Start(&dis.Contract{}); err != nil {
		log.Fatal(err)
	}
}
