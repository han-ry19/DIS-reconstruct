package dns

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"net"
	"os"
	"testing"
)

// Use an isolated Redis instance: this test owns the DIS keys in that instance.
func TestLiveRedisProjectionRecovery(t *testing.T) {
	address := os.Getenv("DIS_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("set DIS_TEST_REDIS_ADDRESS to an isolated Redis instance")
	}
	c, e := ConnectRedis(address)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	keys := []any{cacheKey, "uid.", "data.", "_dis:identities", "_dis:data", "_dis:authorizations", "_dis:applied"}
	if _, e = c.Do("DEL", keys...); e != nil {
		t.Fatal(e)
	}
	if _, err := CheckProjection(address, ""); err == nil {
		t.Fatal("missing checkpoint passed readiness")
	}
	s := fixture()
	p := ProjectionCheckpoint{State: s, Block: 10, Transaction: "first"}
	if e = SaveProjection(address, p, true); e != nil {
		t.Fatal(e)
	}
	checked, err := CheckProjection(address, "")
	if err != nil || checked.Block != p.Block || checked.Transaction != p.Transaction || checked.State.Sequence != s.Sequence {
		t.Fatal("cached projection failed readiness", checked, err)
	}
	metadataOnly, exists, err := LoadCheckpoint(address)
	if err != nil || !exists || metadataOnly.Block != p.Block || metadataOnly.State.Identities != nil {
		t.Fatal("checkpoint read scanned business state", metadataOnly, err)
	}
	recovered, ok, e := LoadProjection(address)
	if e != nil || !ok {
		t.Fatal(e)
	}
	a, _ := Digest(s)
	b, _ := Digest(recovered.State)
	if a != b || recovered.Block != 10 {
		t.Fatal("full recovery changed state")
	}
	id := domain.Identity{ID: "extra.uid", Kind: "user", PublicKey: s.Identities["alice.uid"].PublicKey}
	change := domain.Change{Domain: s.Domain, Sequence: p.State.Sequence + 1, RequestKey: "extra.uid|register", Operation: "identity.register", Identity: &id}
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	p.Change = &change
	p.Block = 11
	p.Transaction = "second"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	recovered, ok, e = LoadProjection(address)
	if e != nil || !ok || recovered.State.Identities[id.ID].ID != id.ID || recovered.Transaction != "second" {
		t.Fatal(recovered, e)
	}
	raw, _ := redis.Bytes(c.Do("GET", cacheKey))
	var metadata ProjectionCheckpoint
	json.Unmarshal(raw, &metadata)
	if metadata.State.Identities != nil {
		t.Fatal("checkpoint contains full business state")
	}
	if _, e = redis.String(c.Do("HGET", "uid.", "extra")); e != nil {
		t.Fatal("incremental DNS record missing", e)
	}
	if _, e = redis.String(c.Do("HGET", "uid.", "alice")); e != nil {
		t.Fatal("unrelated DNS record removed", e)
	}
	if _, e = redis.String(c.Do("HGET", "data.", "sample")); e != nil {
		t.Fatal("unrelated zone removed", e)
	}
	stale := recovered
	stale.State.Sequence--
	if e = SaveProjection(address, stale, true); e == nil {
		t.Fatal("stale publication accepted")
	}
	// A matching AXFR passes; a consistent but stale DNS server must fail after publication advances.
	zones, err := Project(p.State)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &mdns.Server{Listener: listener, Handler: mdns.HandlerFunc(func(w mdns.ResponseWriter, request *mdns.Msg) {
		records := zones[request.Question[0].Name]
		if len(records) == 0 {
			return
		}
		var soa mdns.RR
		for _, rr := range records {
			if rr.Header().Rrtype == mdns.TypeSOA {
				soa = rr
				break
			}
		}
		ordered := []mdns.RR{soa}
		for _, rr := range records {
			if rr.Header().Rrtype != mdns.TypeSOA {
				ordered = append(ordered, rr)
			}
		}
		ordered = append(ordered, soa)
		envelopes := make(chan *mdns.Envelope, 1)
		envelopes <- &mdns.Envelope{RR: ordered}
		close(envelopes)
		if err := new(mdns.Transfer).Out(w, request, envelopes); err != nil {
			t.Error(err)
		}
	})}
	started := make(chan struct{})
	server.NotifyStartedFunc = func() { close(started) }
	go server.ActivateAndServe()
	<-started
	defer server.Shutdown()
	if _, err := CheckProjection(address, listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	p.State.Sequence++
	p.Block++
	p.Change = nil
	if err := SaveProjection(address, p, true); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckProjection(address, listener.Addr().String()); err == nil {
		t.Fatal("stale authoritative DNS passed readiness")
	}
	p.Change = nil
	p.Transaction = "second"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
}
