package main

import (
	"flag"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"log"
)

func main() {
	p := flag.String("fabric-config", "runtime/fabric/gateway.json", "Fabric gateway configuration")
	flag.Parse()
	b, e := fabric.Open(*p)
	if e != nil {
		log.Fatal(e)
	}
	defer b.Close()
	if e = b.MigrateState(); e != nil {
		log.Fatal(e)
	}
	log.Print("ledger migration committed")
}
