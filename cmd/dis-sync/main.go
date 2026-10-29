package main

import (
	"encoding/json"
	"flag"
	"github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"log"
	"os"
	"time"
)

func main() {
	profile := flag.String("fabric-config", "", "Fabric gateway configuration")
	statePath := flag.String("state", "runtime/state.json", "Development state file")
	output := flag.String("out", "runtime/dns", "DNS publication directory")
	interval := flag.Duration("interval", time.Second, "Synchronization interval")
	once := flag.Bool("once", false, "Publish one snapshot and exit")
	verify := flag.String("verify", "", "Authoritative DNS address for complete AXFR verification")
	flag.Parse()
	if *interval <= 0 {
		log.Fatal("interval must be positive")
	}
	var view func() (domain.State, error)
	if *profile != "" {
		b, e := fabric.Open(*profile)
		if e != nil {
			log.Fatal(e)
		}
		defer b.Close()
		view = b.View
	} else {
		view = func() (domain.State, error) {
			raw, e := os.ReadFile(*statePath)
			if e != nil {
				return domain.State{}, e
			}
			var state domain.State
			e = json.Unmarshal(raw, &state)
			return state, e
		}
	}
	run := func() error {
		s, e := view()
		if e != nil {
			return e
		}
		if e = dns.Publish(s, *output); e != nil {
			return e
		}
		if *verify != "" {
			deadline := time.Now().Add(15 * time.Second)
			for {
				e = dns.Verify(s, *verify)
				if e == nil {
					break
				}
				if time.Now().After(deadline) {
					return e
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
		log.Printf("published sequence %d", s.Sequence)
		return nil
	}
	if *once {
		if e := run(); e != nil {
			log.Fatal(e)
		}
		return
	}
	for {
		if e := run(); e != nil {
			log.Printf("synchronization: %v", e)
		}
		time.Sleep(*interval)
	}
}
