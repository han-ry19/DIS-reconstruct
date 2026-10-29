package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"os"
	"time"
)

func main() {
	profile := flag.String("fabric-config", "runtime/fabric/gateway.json", "Fabric gateway configuration")
	wait := flag.Duration("wait", 45*time.Second, "Readiness timeout")
	flag.Parse()
	b, e := fabric.Open(*profile)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer b.Close()
	deadline := time.Now().Add(*wait)
	for {
		e = b.CheckEndorsement()
		if e == nil {
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		time.Sleep(300 * time.Millisecond)
	}
	state, e := b.View()
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ready", "sequence": state.Sequence, "domain": state.Domain})
}
