// Package disredis adapts the original DIS Redis record format to CoreDNS 1.12.
package disredis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/gomodule/redigo/redis"
	mdns "github.com/miekg/dns"
	"net"
	"strings"
	"time"
)

type Handler struct {
	Next  plugin.Handler
	Pool  *redis.Pool
	Zones []string
}

func init() { plugin.Register("disredis", setup) }
func setup(c *caddy.Controller) error {
	address := "127.0.0.1:28679"
	for c.Next() {
		for c.NextBlock() {
			switch c.Val() {
			case "address":
				if !c.NextArg() {
					return c.ArgErr()
				}
				address = c.Val()
			default:
				return c.Errf("unknown property %s", c.Val())
			}
		}
	}
	h := &Handler{Zones: []string{"uid.", "data."}}
	h.Pool = &redis.Pool{MaxIdle: 4, MaxActive: 64, Wait: true, Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", address, redis.DialConnectTimeout(2*time.Second), redis.DialReadTimeout(5*time.Second), redis.DialWriteTimeout(5*time.Second))
	}}
	c.OnShutdown(h.Pool.Close)
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler { h.Next = next; return h })
	return nil
}
func (h *Handler) Name() string { return "disredis" }
func (h *Handler) records(c redis.Conn, zone, label string) ([]mdns.RR, error) {
	b, e := redis.Bytes(c.Do("HGET", zone, label))
	if e != nil {
		return nil, e
	}
	name := zone
	if label != "@" {
		name = label + "." + zone
	}
	return decode(name, b)
}
func (h *Handler) ServeDNS(ctx context.Context, w mdns.ResponseWriter, r *mdns.Msg) (int, error) {
	if len(r.Question) != 1 {
		return mdns.RcodeFormatError, nil
	}
	q := r.Question[0]
	name := strings.ToLower(q.Name)
	zone := plugin.Zones(h.Zones).Matches(name)
	if zone == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}
	c := h.Pool.Get()
	defer c.Close()
	label := strings.TrimSuffix(name, "."+zone)
	if name == zone {
		label = "@"
	}
	if q.Qtype == mdns.TypeAXFR {
		if _, ok := w.RemoteAddr().(*net.TCPAddr); !ok {
			return mdns.RcodeRefused, nil
		}
		values, e := redis.StringMap(c.Do("HGETALL", zone))
		if e != nil {
			return mdns.RcodeServerFailure, e
		}
		all := []mdns.RR{}
		var soa mdns.RR
		for field, raw := range values {
			fqdn := zone
			if field != "@" {
				fqdn = field + "." + zone
			}
			rows, e := decode(fqdn, []byte(raw))
			if e != nil {
				return mdns.RcodeServerFailure, e
			}
			for _, rr := range rows {
				if rr.Header().Rrtype == mdns.TypeSOA {
					soa = rr
				} else {
					all = append(all, rr)
				}
			}
		}
		if soa == nil {
			return mdns.RcodeServerFailure, fmt.Errorf("missing SOA")
		}
		all = append([]mdns.RR{soa}, append(all, soa)...)

		batches := [][]mdns.RR{}
		batch := []mdns.RR{}
		size := 0
		for _, rr := range all {
			length := mdns.Len(rr)
			if length > 48000 {
				return mdns.RcodeServerFailure, fmt.Errorf("record exceeds AXFR message limit")
			}
			if size+length > 48000 {
				batches = append(batches, batch)
				batch = nil
				size = 0
			}
			batch = append(batch, rr)
			size += length
		}
		if len(batch) > 0 {
			batches = append(batches, batch)
		}
		envelopes := make(chan *mdns.Envelope, len(batches))
		for _, rows := range batches {
			envelopes <- &mdns.Envelope{RR: rows}
		}
		close(envelopes)
		t := new(mdns.Transfer)
		e = t.Out(w, r, envelopes)
		if e != nil {
			return mdns.RcodeServerFailure, e
		}
		w.Hijack()
		return mdns.RcodeSuccess, nil
	}
	rows, e := h.records(c, zone, label)
	reply := new(mdns.Msg)
	reply.SetReply(r)
	reply.Authoritative = true
	if e == redis.ErrNil {
		reply.Rcode = mdns.RcodeNameError
	} else if e != nil {
		return mdns.RcodeServerFailure, e
	} else {
		for _, rr := range rows {
			if rr.Header().Rrtype == q.Qtype || q.Qtype == mdns.TypeANY {
				reply.Answer = append(reply.Answer, rr)
			}
		}
	}
	if len(reply.Answer) == 0 {
		apex, ae := h.records(c, zone, "@")
		if ae == nil {
			for _, rr := range apex {
				if rr.Header().Rrtype == mdns.TypeSOA {
					reply.Ns = append(reply.Ns, rr)
				}
			}
		}
	}

	if _, udp := w.RemoteAddr().(*net.UDPAddr); udp {
		size := mdns.MinMsgSize
		if opt := r.IsEdns0(); opt != nil {
			size = int(opt.UDPSize())
			if size < mdns.MinMsgSize {
				size = mdns.MinMsgSize
			}
			if size > 4096 {
				size = 4096
			}
		}
		reply.Truncate(size)
	}
	return mdns.RcodeSuccess, w.WriteMsg(reply)
}
func decode(name string, b []byte) ([]mdns.RR, error) {
	var row struct {
		A []struct {
			Ttl uint32
			Ip  string
		}
		NS []struct {
			Ttl  uint32
			Host string
		}
		TXT []struct {
			Ttl  uint32
			Text string
		}
		CERT []struct {
			Ttl          uint32
			Type, Keytag uint16
			Algorithm    uint8
			Certificate  string
		}
		RP []struct {
			Ttl       uint32
			Mbox, Txt string
		}
		URI []struct {
			Ttl              uint32
			Priority, Weight uint16
			Target           string
		}
		SOA struct {
			Ttl                                    uint32
			Ns, MBox                               string
			Serial, Refresh, Retry, Expire, MinTtl uint32
		}
	}
	if e := json.Unmarshal(b, &row); e != nil {
		return nil, e
	}
	out := []mdns.RR{}
	header := func(t uint16, ttl uint32) mdns.RR_Header {
		return mdns.RR_Header{Name: name, Rrtype: t, Class: mdns.ClassINET, Ttl: ttl}
	}
	for _, v := range row.A {
		out = append(out, &mdns.A{Hdr: header(mdns.TypeA, v.Ttl), A: net.ParseIP(v.Ip).To4()})
	}
	for _, v := range row.NS {
		out = append(out, &mdns.NS{Hdr: header(mdns.TypeNS, v.Ttl), Ns: v.Host})
	}
	for _, v := range row.TXT {
		chunks := []string{}
		text := v.Text
		for len(text) > 200 {
			chunks = append(chunks, text[:200])
			text = text[200:]
		}
		chunks = append(chunks, text)
		out = append(out, &mdns.TXT{Hdr: header(mdns.TypeTXT, v.Ttl), Txt: chunks})
	}
	for _, v := range row.CERT {
		out = append(out, &mdns.CERT{Hdr: header(mdns.TypeCERT, v.Ttl), Type: v.Type, KeyTag: v.Keytag, Algorithm: v.Algorithm, Certificate: v.Certificate})
	}
	for _, v := range row.RP {
		out = append(out, &mdns.RP{Hdr: header(mdns.TypeRP, v.Ttl), Mbox: v.Mbox, Txt: v.Txt})
	}
	for _, v := range row.URI {
		out = append(out, &mdns.URI{Hdr: header(mdns.TypeURI, v.Ttl), Priority: v.Priority, Weight: v.Weight, Target: v.Target})
	}
	v := row.SOA
	if v.Ns != "" {
		out = append(out, &mdns.SOA{Hdr: header(mdns.TypeSOA, v.Ttl), Ns: v.Ns, Mbox: v.MBox, Serial: v.Serial, Refresh: v.Refresh, Retry: v.Retry, Expire: v.Expire, Minttl: v.MinTtl})
	}
	return out, nil
}
