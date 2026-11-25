package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"log"
	"os"
	"time"
)

func main() {
	profile := flag.String("fabric-config", "", "Fabric gateway configuration")
	redis := flag.String("redis", "", "Redis serving projection address")
	authoritative := flag.String("verify", "", "Authoritative DNS address for optional AXFR verification")
	ns := flag.String("ns-address", "127.0.0.1", "Advertised nameserver IPv4 address")
	timeout := flag.Duration("timeout", 30*time.Minute, "Maximum audit duration")
	flag.Parse()
	if *profile == "" || *redis == "" || *timeout <= 0 {
		log.Fatal("audit requires -fabric-config, -redis, and a positive timeout")
	}
	b, err := fabric.Open(*profile)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	state, height, err := b.Rebuild(ctx)
	if err != nil {
		log.Fatal(err)
	}
	p, ok, err := dns.LoadProjection(*redis)
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		log.Fatal("missing Redis projection")
	}
	expected, err := dns.Digest(state)
	if err != nil {
		log.Fatal(err)
	}
	actual, err := dns.Digest(p.State)
	if err != nil {
		log.Fatal(err)
	}
	if expected != actual {
		log.Fatalf("audit mismatch at captured channel height %d: recovered sequence %d, published sequence %d", height, state.Sequence, p.State.Sequence)
	}
	if *authoritative != "" {
		checked, err := dns.CheckProjection(*redis, *authoritative, *ns)
		if err != nil {
			log.Fatal(err)
		}
		if checked.State.Domain != p.State.Domain || checked.State.Sequence != p.State.Sequence || checked.Block != p.Block || checked.Transaction != p.Transaction {
			log.Fatal("projection advanced during audit; retry after management writes stop")
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "matched", "domain": state.Domain, "sequence": state.Sequence, "channel_height": height, "checkpoint_block": p.Block, "checkpoint_transaction": p.Transaction})
}
