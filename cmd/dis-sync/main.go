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

func verifyState(s domain.State, address string, nsAddress ...string) error {
	if address == "" {
		return nil
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		e := dns.Verify(s, address, nsAddress...)
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
	nsAddress := flag.String("ns-address", "127.0.0.1", "Advertised authoritative nameserver IPv4 address")
	profile := flag.String("fabric-config", "", "Fabric gateway configuration")
	statePath := flag.String("state", "runtime/state.json", "Development state file")
	output := flag.String("out", "runtime/dns", "Zone-file export directory")
	address := flag.String("redis", "", "Redis address for event-driven publication")
	interval := flag.Duration("interval", time.Second, "Development polling or event reconnect interval")
	bootstrap := flag.Bool("bootstrap", false, "Reuse a persistent projection or rebuild it when absent")
	once := flag.Bool("once", false, "Recover and publish committed state once, then exit")
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
		view = func() (domain.State, error) { s, _, err := b.Rebuild(context.Background()); return s, err }
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
		var s domain.State
		var height uint64
		var e error
		if b != nil {
			s, height, e = b.Rebuild(context.Background())
		} else {
			s, e = view()
		}
		if e != nil {
			return e
		}
		if *address != "" {
			p := dns.ProjectionCheckpoint{State: s, Block: height}
			if e = dns.SaveProjection(*address, p, true, *nsAddress); e != nil {
				return e
			}
		}
		if e = dns.Publish(s, *output, *nsAddress); e != nil {
			return e
		}
		if e = verifyState(s, *verify, *nsAddress); e != nil {
			return e
		}
		log.Printf("published recovered state sequence %d next block %d", s.Sequence, height)
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
		if e = verifyState(s, *verify, *nsAddress); e != nil {
			log.Fatal(e)
		}
		log.Printf("verified event projection sequence %d", s.Sequence)
		return
	}
	if *bootstrap {
		if b == nil || *address == "" {
			log.Fatal("bootstrap requires -fabric-config and -redis")
		}
		p, ok, e := dns.LoadProjection(*address)
		if e != nil {
			log.Fatal(e)
		}
		if ok {
			if e = dns.SaveProjection(*address, p, true, *nsAddress); e != nil {
				log.Fatal(e)
			}
			if e = dns.Publish(p.State, *output, *nsAddress); e != nil {
				log.Fatal(e)
			}
			if e = verifyState(p.State, *verify, *nsAddress); e != nil {
				log.Fatal(e)
			}
			log.Printf("reused projection sequence %d checkpoint block %d", p.State.Sequence, p.Block)
			return
		}
	}
	if *once || *bootstrap {
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
		if e := follow(b, *address, *output, *verify, *nsAddress); e != nil {
			log.Printf("event synchronization: %v", e)
		}
		time.Sleep(*interval)
	}
}
func follow(b *fabric.Backend, address, output, verify, nsAddress string) error {
	p, ok, e := dns.LoadProjection(address)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !ok {
		p.State, p.Block, e = b.Rebuild(ctx)
		if e != nil {
			return e
		}
		if e = dns.SaveProjection(address, p, true, nsAddress); e != nil {
			return e
		}
		if e = dns.Publish(p.State, output, nsAddress); e != nil {
			return e
		}
		log.Printf("recovered projection sequence %d next block %d", p.State.Sequence, p.Block)
	}
	events, e := b.Events(ctx, p)
	if e != nil {
		return e
	}
	log.Printf("following events from block %d after %s", p.Block, p.Transaction)
	for event := range events {
		if event.EventName != domain.ZoneUpdateEvent {
			return fmt.Errorf("unsupported DIS event %s", event.EventName)
		}
		var change domain.Change
		if e = json.Unmarshal(event.Payload, &change); e != nil {
			return e
		}
		change.Sequence = p.State.Sequence + 1
		if e = dns.ApplyChange(&p.State, change); e != nil {
			return e
		}
		p.Change = &change
		p.Block = event.BlockNumber
		p.Transaction = event.TransactionID
		if e = dns.SaveProjection(address, p, false, nsAddress); e != nil {
			return e
		}
		if e = verifyState(p.State, verify, nsAddress); e != nil {
			return e
		}
		log.Printf("published event sequence %d block %d transaction %s", p.State.Sequence, p.Block, p.Transaction)
	}
	return fmt.Errorf("event stream closed")
}
