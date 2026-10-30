package dns

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"strings"
	"time"
)

const cacheKey = "_dis:projection:v1"

// ProjectionCheckpoint couples the published business state and event position.
type ProjectionCheckpoint struct {
	Digest      string         `json:"digest"`
	Change      *domain.Change `json:"-"`
	State       domain.State   `json:"state"`
	Block       uint64         `json:"block"`
	Transaction string         `json:"transaction"`
}

func (p ProjectionCheckpoint) BlockNumber() uint64   { return p.Block }
func (p ProjectionCheckpoint) TransactionID() string { return p.Transaction }
func ConnectRedis(address string) (redis.Conn, error) {
	return redis.Dial("tcp", address, redis.DialConnectTimeout(5*time.Second), redis.DialReadTimeout(10*time.Second), redis.DialWriteTimeout(10*time.Second))
}
func LoadProjection(address string) (ProjectionCheckpoint, bool, error) {
	var p ProjectionCheckpoint
	c, e := ConnectRedis(address)
	if e != nil {
		return p, false, e
	}
	defer c.Close()
	// WATCH makes the metadata and object-cache reads one consistent recovery view.
	if _, e = c.Do("WATCH", cacheKey); e != nil {
		return p, false, e
	}
	b, e := redis.Bytes(c.Do("GET", cacheKey))
	if e == redis.ErrNil {
		return p, false, nil
	}
	if e != nil {
		return p, false, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, false, e
	}
	if p.State.Identities != nil {
		return p, true, nil
	}
	s := domain.NewState(p.State.Domain)
	s.Sequence = p.State.Sequence
	s.ReceiptSequence = p.State.ReceiptSequence
	load := func(key string, consume func(string, []byte) error) error {
		values, e := redis.StringMap(c.Do("HGETALL", key))
		if e != nil {
			return e
		}
		for k, v := range values {
			if e = consume(k, []byte(v)); e != nil {
				return e
			}
		}
		return nil
	}
	if e = load("_dis:identities", func(k string, b []byte) error {
		var v domain.Identity
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Identities[k] = v
		return nil
	}); e != nil {
		return p, false, e
	}
	if e = load("_dis:data", func(k string, b []byte) error {
		var v domain.DataIdentifier
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Data[k] = v
		return nil
	}); e != nil {
		return p, false, e
	}
	if e = load("_dis:authorizations", func(k string, b []byte) error {
		var v domain.AuthorizationRecord
		if e := json.Unmarshal(b, &v); e != nil {
			return e
		}
		s.Authorizations[k] = v
		return nil
	}); e != nil {
		return p, false, e
	}
	if s.ReceiptSequence {
		s.AppliedRequests = map[string]bool{}
		if e = load("_dis:applied", func(k string, b []byte) error { s.AppliedRequests[k] = true; return nil }); e != nil {
			return p, false, e
		}
	}
	if _, e = c.Do("MULTI"); e != nil {
		return p, false, e
	}
	if e = c.Send("PING"); e != nil {
		return p, false, e
	}
	if _, e = redis.Values(c.Do("EXEC")); e != nil {
		return p, false, e
	}
	p.State = s
	return p, true, nil
}

// RedisRecord retains the original DIS plugin's per-label JSON representation.
type RedisRecord map[string]any

func encodeRecords(s domain.State) (map[string]map[string]string, error) {
	zones, e := Project(s)
	if e != nil {
		return nil, e
	}
	out := map[string]map[string]string{}
	for zone, rrs := range zones {
		labels := map[string]RedisRecord{}
		for _, rr := range rrs {
			label := strings.TrimSuffix(rr.Header().Name, "."+zone)
			if rr.Header().Name == zone {
				label = "@"
			}
			row := labels[label]
			if row == nil {
				row = RedisRecord{}
				labels[label] = row
			}
			kind := strings.ToLower(mdns.TypeToString[rr.Header().Rrtype])
			ttl := rr.Header().Ttl
			var value any
			switch v := rr.(type) {
			case *mdns.SOA:
				value = map[string]any{"ttl": ttl, "ns": v.Ns, "MBox": v.Mbox, "serial": v.Serial, "refresh": v.Refresh, "retry": v.Retry, "expire": v.Expire, "minttl": v.Minttl}
				row[kind] = value
				continue
			case *mdns.NS:
				value = map[string]any{"ttl": ttl, "host": v.Ns}
			case *mdns.A:
				value = map[string]any{"ttl": ttl, "ip": v.A.String()}
			case *mdns.TXT:
				value = map[string]any{"ttl": ttl, "text": strings.Join(v.Txt, "")}
			case *mdns.CERT:
				value = map[string]any{"ttl": ttl, "type": v.Type, "keytag": v.KeyTag, "algorithm": v.Algorithm, "certificate": v.Certificate}
			case *mdns.RP:
				value = map[string]any{"ttl": ttl, "mbox": v.Mbox, "txt": v.Txt}
			case *mdns.URI:
				value = map[string]any{"ttl": ttl, "priority": v.Priority, "weight": v.Weight, "target": v.Target}
			default:
				return nil, fmt.Errorf("unsupported projection record %T", rr)
			}
			values, _ := row[kind].([]any)
			row[kind] = append(values, value)
		}
		encoded := map[string]string{}
		for label, row := range labels {
			b, e := json.Marshal(row)
			if e != nil {
				return nil, e
			}
			encoded[label] = string(b)
		}
		out[zone] = encoded
	}
	return out, nil
}

// SaveProjection atomically publishes records, cached objects, and event position.
func SaveProjection(address string, p ProjectionCheckpoint, full bool) error {
	c, e := ConnectRedis(address)
	if e != nil {
		return e
	}
	defer c.Close()
	if _, e = c.Do("WATCH", cacheKey); e != nil {
		return e
	}
	raw, e := redis.Bytes(c.Do("GET", cacheKey))
	exists := e == nil
	if e != nil && e != redis.ErrNil {
		return e
	}
	var old ProjectionCheckpoint
	if exists {
		if e = json.Unmarshal(raw, &old); e != nil {
			return e
		}
		if old.State.Domain != p.State.Domain || old.State.Sequence > p.State.Sequence {
			return fmt.Errorf("stale Redis projection")
		}
		if old.State.Sequence == p.State.Sequence {
			expected := old.Digest
			if expected == "" {
				expected, _ = Digest(old.State)
			}
			actual, _ := Digest(p.State)
			if expected != actual {
				return fmt.Errorf("conflicting Redis projection")
			}
		}
	}
	full = full || !exists || old.State.Identities != nil
	delta := domain.NewState(p.State.Domain)
	delta.Sequence = p.State.Sequence
	if full {
		delta = p.State
	} else if p.Change != nil {
		change := p.Change
		if change.Identity != nil {
			delta.Identities[change.Identity.ID] = *change.Identity
		}
		if change.Data != nil {
			delta.Data[change.Data.ID] = *change.Data
		}
		if change.Authorization != nil {
			a := *change.Authorization
			delta.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
		}
	}
	next, e := encodeRecords(delta)
	if e != nil {
		return e
	}
	p.Digest, e = Digest(p.State)
	if e != nil {
		return e
	}
	metadata := p
	metadata.State = domain.State{Domain: p.State.Domain, Sequence: p.State.Sequence, ReceiptSequence: p.State.ReceiptSequence}
	b, e := json.Marshal(metadata)
	if e != nil {
		return e
	}
	if e = c.Send("MULTI"); e != nil {
		return e
	}
	if full || p.Change != nil {
		for _, zone := range []string{"uid.", "data."} {
			target := zone
			if full {
				target = "_dis:staging:" + zone
				if e = c.Send("DEL", target); e != nil {
					return e
				}
			}
			for label, value := range next[zone] {
				if e = c.Send("HSET", target, label, value); e != nil {
					return e
				}
			}
			if full {
				if e = c.Send("RENAME", target, zone); e != nil {
					return e
				}
			}
		}
		objects := []struct {
			key    string
			values map[string]any
		}{{"_dis:identities", map[string]any{}}, {"_dis:data", map[string]any{}}, {"_dis:authorizations", map[string]any{}}}
		for k, v := range delta.Identities {
			objects[0].values[k] = v
		}
		for k, v := range delta.Data {
			objects[1].values[k] = v
		}
		for k, v := range delta.Authorizations {
			objects[2].values[k] = v
		}
		for _, object := range objects {
			if full {
				if e = c.Send("DEL", object.key); e != nil {
					return e
				}
			}
			for k, v := range object.values {
				encoded, e := json.Marshal(v)
				if e != nil {
					return e
				}
				if e = c.Send("HSET", object.key, k, encoded); e != nil {
					return e
				}
			}
		}
	}
	if full {
		if e = c.Send("DEL", "_dis:applied"); e != nil {
			return e
		}
		for k := range p.State.AppliedRequests {
			if e = c.Send("HSET", "_dis:applied", k, "1"); e != nil {
				return e
			}
		}
	} else if p.Change != nil && p.Change.RequestKey != "" {
		if e = c.Send("HSET", "_dis:applied", p.Change.RequestKey, "1"); e != nil {
			return e
		}
	}
	if e = c.Send("SET", cacheKey, b); e != nil {
		return e
	}
	replies, e := redis.Values(c.Do("EXEC"))
	if e != nil {
		return e
	}
	for _, r := range replies {
		if er, ok := r.(redis.Error); ok {
			return er
		}
	}
	return nil
}
func ApplyChange(s *domain.State, c domain.Change) error {
	if c.Domain != s.Domain {
		return fmt.Errorf("event domain mismatch")
	}
	if c.RequestKey != "" {
		if !s.ReceiptSequence {
			return fmt.Errorf("receipt event requires receipt-count snapshot")
		}
		if s.AppliedRequests[c.RequestKey] {
			return nil
		}
		c.Sequence = s.Sequence + 1
	}
	if c.Sequence <= s.Sequence {
		return nil
	}
	if c.Sequence != s.Sequence+1 {
		return fmt.Errorf("event sequence gap")
	}
	switch c.Operation {
	case "identity.register":
		if c.Identity == nil {
			return fmt.Errorf("missing identity")
		}
		s.Identities[c.Identity.ID] = *c.Identity
	case "data.register":
		if c.Data == nil {
			return fmt.Errorf("missing data")
		}
		s.Data[c.Data.ID] = *c.Data
	case "authorization.request", "authorization.decide":
		if c.Authorization == nil {
			return fmt.Errorf("missing authorization")
		}
		a := *c.Authorization
		s.Authorizations[domain.AuthorizationKey(a.DataID, a.OwnerID, a.Recipient)] = a
	default:
		return fmt.Errorf("unknown event operation")
	}
	s.Sequence = c.Sequence
	if c.RequestKey != "" {
		if s.AppliedRequests == nil {
			s.AppliedRequests = map[string]bool{}
		}
		s.AppliedRequests[c.RequestKey] = true
	}
	return nil
}
