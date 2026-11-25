package dns

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gomodule/redigo/redis"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
)

func TestUpdateEventsReplaceRecords(t *testing.T) {
	s := fixture()
	d := s.Data["sample.data"]
	d.Address = "https://example.org/new"
	d.OwnerID = "bob.uid"
	d.DataSig = "new signature"
	if e := ApplyChange(&s, domain.Change{Domain: s.Domain, Sequence: s.Sequence + 1, Operation: "data.update", Data: &d}); e != nil {
		t.Fatal(e)
	}
	rows, e := encodeRecords(s)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(rows["data."]["sample"], "https://example.org/new") || !strings.Contains(rows["data."]["sample"], "bob.uid.") {
		t.Fatal(rows)
	}
	if _, ok := rows["data."]["_profile.sample"]; ok {
		t.Fatal("profile unexpectedly published as a DNS RR")
	}
	old := s.Authorizations["sample.data|alice.uid|app.uid"]
	a := old
	a.Recipient = "other.uid"
	a.Status = domain.AuthorizationRejected
	c := domain.Change{Domain: s.Domain, Sequence: s.Sequence + 1, Operation: "authorization.update", Authorization: &a, RemovedAuthorizations: []domain.AuthorizationRecord{old}}
	if e := ApplyChange(&s, c); e != nil {
		t.Fatal(e)
	}
	rows, e = encodeRecords(s)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := rows["data."]["app.sample"]; ok {
		t.Fatal("old DNS label retained")
	}
	if !strings.Contains(rows["data."]["other.sample"], "rejected") {
		t.Fatal("replacement authorization missing")
	}
	before, _ := Digest(s)
	if e := ApplyChange(&s, c); e != nil {
		t.Fatal(e)
	}
	after, _ := Digest(s)
	if before != after {
		t.Fatal("replay changed state")
	}
}

func TestLiveRedisRecipientMoveAndUpdates(t *testing.T) {
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
	s := fixture()
	p := ProjectionCheckpoint{State: s, Block: 10, Transaction: "setup"}
	if e = SaveProjection(address, p, true); e != nil {
		t.Fatal(e)
	}
	old := s.Authorizations["sample.data|alice.uid|app.uid"]
	a := old
	a.Recipient = "other.uid"
	change := domain.Change{Domain: s.Domain, Sequence: p.State.Sequence + 1, RequestKey: "alice.uid|move", Operation: "authorization.update", Authorization: &a, RemovedAuthorizations: []domain.AuthorizationRecord{old}}
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	p.Change = &change
	p.Block++
	p.Transaction = "move"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	if exists, _ := redis.Bool(c.Do("HEXISTS", "data.", "app.sample")); exists {
		t.Fatal("old DNS label survived HDEL")
	}
	if exists, _ := redis.Bool(c.Do("HEXISTS", "_dis:authorizations", "sample.data|alice.uid|app.uid")); exists {
		t.Fatal("old cached grant survived HDEL")
	}
	if exists, _ := redis.Bool(c.Do("HEXISTS", "data.", "other.sample")); !exists {
		t.Fatal("new DNS label missing")
	}
	// Replay the exact committed event; deleted keys must stay deleted.
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	d := p.State.Data["sample.data"]
	d.Address = "https://example.org/new"
	change = domain.Change{Domain: s.Domain, Sequence: p.State.Sequence + 1, RequestKey: "alice.uid|address", Operation: "data.update", Data: &d}
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	p.Change = &change
	p.Block++
	p.Transaction = "address"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	a.Status = domain.AuthorizationRejected
	change = domain.Change{Domain: s.Domain, Sequence: p.State.Sequence + 1, RequestKey: "alice.uid|revoke", Operation: "authorization.update", Authorization: &a}
	if e = ApplyChange(&p.State, change); e != nil {
		t.Fatal(e)
	}
	p.Change = &change
	p.Block++
	p.Transaction = "revoke"
	if e = SaveProjection(address, p, false); e != nil {
		t.Fatal(e)
	}
	recovered, ok, e := LoadProjection(address)
	if e != nil || !ok {
		t.Fatal(e)
	}
	expected, _ := Digest(p.State)
	actual, _ := Digest(recovered.State)
	if expected != actual || recovered.Block != p.Block || recovered.Transaction != "revoke" {
		t.Fatal("recovery differs from published state")
	}
	full, e := encodeRecords(recovered.State)
	if e != nil {
		t.Fatal(e)
	}
	for _, zone := range []string{"uid.", "data."} {
		rows, e := redis.StringMap(c.Do("HGETALL", zone))
		if e != nil {
			t.Fatal(e)
		}
		want, _ := json.Marshal(full[zone])
		got, _ := json.Marshal(rows)
		if string(want) != string(got) {
			t.Fatalf("incremental %s differs from full projection", zone)
		}
	}
	if e = SaveProjection(address, ProjectionCheckpoint{State: s, Block: 10}, true); e == nil {
		t.Fatal("stale full snapshot restored old records")
	}
}
