package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/dns"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	"github.com/han-ry19/DIS-reconstruct/internal/fabric"
	"log"
	"os"
	"time"
)

func verifyState(s domain.State, address string) error {
	if address == "" {
		return nil
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		e := dns.Verify(s, address)
		if e == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return e
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func main() {
	profile := flag.String("fabric-config", "", "Fabric gateway configuration")
	statePath := flag.String("state", "runtime/state.json", "Development state file")
	output := flag.String("out", "runtime/dns", "Zone-file export directory")
	address := flag.String("redis", "", "Redis address for event-driven publication")
	interval := flag.Duration("interval", time.Second, "Development polling or event reconnect interval")
	once := flag.Bool("once", false, "Publish one full snapshot and exit")
	verify := flag.String("verify", "", "Authoritative DNS address for complete AXFR verification")
	check := flag.Bool("check", false, "Verify Redis and authoritative DNS against Fabric without publishing")
	flag.Parse()
	if *interval <= 0 {
		log.Fatal("interval must be positive")
	}
	var view func() (domain.State, error)
	var b *fabric.Backend
	if *profile != "" {
		var e error
		b, e = fabric.Open(*profile)
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
			var s domain.State
			e = json.Unmarshal(raw, &s)
			return s, e
		}
	}
	full := func() error {
		s, e := view()
		if e != nil {
			return e
		}
		if *address != "" {
			old, ok, e := dns.LoadProjection(*address)
			if e != nil {
				return e
			}
			p := dns.ProjectionCheckpoint{State: s}
			if ok {
				p.Block = old.Block
				p.Transaction = old.Transaction
			}
			if e = dns.SaveProjection(*address, p, true); e != nil {
				return e
			}
		}
		if e = dns.Publish(s, *output); e != nil {
			return e
		}
		if e = verifyState(s, *verify); e != nil {
			return e
		}
		log.Printf("published full snapshot sequence %d", s.Sequence)
		return nil
	}

	if *check {
		if *address == "" {
			log.Fatal("check requires -redis")
		}
		s, e := view()
		if e != nil {
			log.Fatal(e)
		}
		p, ok, e := dns.LoadProjection(*address)
		if e != nil {
			log.Fatal(e)
		}
		if !ok {
			log.Fatal("missing Redis projection")
		}
		expected, _ := dns.Digest(s)
		actual, _ := dns.Digest(p.State)
		if expected != actual {
			log.Fatal("Redis projection does not match committed state")
		}
		if e = verifyState(s, *verify); e != nil {
			log.Fatal(e)
		}
		log.Printf("verified event projection sequence %d", s.Sequence)
		return
	}
	if *once {
		if e := full(); e != nil {
			log.Fatal(e)
		}
		return
	}
	if b == nil || *address == "" {
		for {
			if e := full(); e != nil {
				log.Printf("synchronization: %v", e)
			}
			time.Sleep(*interval)
		}
	}
	for {
		if e := follow(b, view, *address, *output, *verify); e != nil {
			log.Printf("event synchronization: %v", e)
		}
		time.Sleep(*interval)
	}
}
func follow(b *fabric.Backend, view func() (domain.State, error), address, output, verify string) error {
	p, ok, e := dns.LoadProjection(address)
	if e != nil {
		return e
	}
	if !ok {
		s, e := view()
		if e != nil {
			return e
		}
		p.State = s
		if e = dns.SaveProjection(address, p, true); e != nil {
			return e
		}
		if e = dns.Publish(s, output); e != nil {
			return e
		}
		log.Printf("bootstrapped event projection sequence %d", s.Sequence)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, e := b.Events(ctx, p)
	if e != nil {
		return e
	}
	log.Printf("following committed events from block %d after %s", p.Block, p.Transaction)
	for event := range events {
		p.Change = nil
		fullUpdate := event.EventName == domain.FullUpdateEvent
		switch event.EventName {
		case domain.ZoneUpdateEvent:
			var change domain.Change
			if e = json.Unmarshal(event.Payload, &change); e != nil {
				return e
			}
			if change.RequestKey != "" && p.State.AppliedRequests[change.RequestKey] {
				// A bootstrap snapshot already contains this committed transition.
				continue
			}
			if (change.RequestKey != "" && !p.State.AppliedRequests[change.RequestKey]) || (change.RequestKey == "" && change.Sequence > p.State.Sequence) {
				p.Change = &change
				if e = dns.ApplyChange(&p.State, change); e != nil {
					// Recover missed updates from one committed snapshot, then resume the stream.
					s, readErr := view()
					if readErr != nil {
						return readErr
					}
					if s.Sequence < change.Sequence {
						return fmt.Errorf("snapshot behind event")
					}
					p.State = s
					fullUpdate = true
					p.Change = nil
				}
			}
		case domain.FullUpdateEvent:
			p.State, e = view()
			if e != nil {
				return e
			}
		default:
			continue
		}
		p.Block = event.BlockNumber
		p.Transaction = event.TransactionID
		if e = dns.SaveProjection(address, p, fullUpdate); e != nil {
			return e
		}
		if e = verifyState(p.State, verify); e != nil {
			return e
		}
		log.Printf("published event sequence %d block %d transaction %s", p.State.Sequence, p.Block, p.Transaction)
	}
	return fmt.Errorf("event stream closed")
}
