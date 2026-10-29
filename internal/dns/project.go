package dns

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"sort"
	"strings"
)

const TTL = 3600

func IdentityName(id string) bool {
	return strings.HasSuffix(id, ".uid") && strings.Count(id, ".") == 1
}
func AuthorizationName(a domain.AuthorizationRecord) string {
	return strings.TrimSuffix(a.Recipient, ".uid") + "." + a.DataID + "."
}
func header(name string, kind uint16) mdns.RR_Header {
	return mdns.RR_Header{Name: mdns.Fqdn(name), Rrtype: kind, Class: mdns.ClassINET, Ttl: TTL}
}
func text(name, value string) *mdns.TXT {
	chunks := []string{}
	for len(value) > 200 {
		chunks = append(chunks, value[:200])
		value = value[200:]
	}
	chunks = append(chunks, value)
	return &mdns.TXT{Hdr: header(name, mdns.TypeTXT), Txt: chunks}
}
func base(zone string, serial uint32) []mdns.RR {
	return []mdns.RR{
		&mdns.SOA{Hdr: header(zone, mdns.TypeSOA), Ns: "ns." + zone, Mbox: "hostmaster." + zone, Serial: serial, Refresh: 60, Retry: 30, Expire: 86400, Minttl: 60},
		&mdns.NS{Hdr: header(zone, mdns.TypeNS), Ns: "ns." + zone},
		&mdns.A{Hdr: header("ns."+zone, mdns.TypeA), A: []byte{127, 0, 0, 1}},
	}
}

// Project implements Dominate's CERT/TXT, RP/URI/TXT, and authorization TXT mapping.
func Project(s domain.State) (map[string][]mdns.RR, error) {
	if s.Sequence >= uint64(^uint32(0)) {
		return nil, fmt.Errorf("DNS serial exhausted")
	}
	serial := uint32(s.Sequence) + 1
	zones := map[string][]mdns.RR{"uid.": base("uid.", serial), "data.": base("data.", serial)}
	for _, id := range s.Identities {
		if !IdentityName(id.ID) {
			return nil, fmt.Errorf("identity outside uid zone: %s", id.ID)
		}
		pub, e := base64.StdEncoding.DecodeString(id.PublicKey)
		if e != nil || len(pub) != 32 {
			return nil, fmt.Errorf("invalid public key")
		}
		// CERT OID format: length, DER OID value (1.3.101.112), and raw Ed25519 key.
		payload := append([]byte{3, 0x2b, 0x65, 0x70}, pub...)
		zones["uid."] = append(zones["uid."], &mdns.CERT{Hdr: header(id.ID, mdns.TypeCERT), Type: 254, KeyTag: 0, Algorithm: 0, Certificate: base64.StdEncoding.EncodeToString(payload)}, text(id.ID, "Ed25519"))
	}
	for _, d := range s.Data {
		if !strings.HasSuffix(d.ID, ".data") || strings.Count(d.ID, ".") != 1 {
			return nil, fmt.Errorf("invalid data name: %s", d.ID)
		}
		zones["data."] = append(zones["data."], &mdns.RP{Hdr: header(d.ID, mdns.TypeRP), Mbox: mdns.Fqdn(d.OwnerID), Txt: "."}, &mdns.URI{Hdr: header(d.ID, mdns.TypeURI), Priority: 0, Weight: 0, Target: d.Address}, text(d.ID, d.DataSig))
		profile, e := json.Marshal(d.Profile)
		if e != nil {
			return nil, e
		}
		zones["data."] = append(zones["data."], text("_profile."+d.ID, string(profile)))
	}
	for _, a := range s.Authorizations {
		if !IdentityName(a.Recipient) {
			return nil, fmt.Errorf("invalid authorization recipient")
		}
		b, e := json.Marshal(a)
		if e != nil {
			return nil, e
		}
		zones["data."] = append(zones["data."], text(AuthorizationName(a), string(b)))
	}
	for _, records := range zones {
		sort.Slice(records, func(i, j int) bool { return records[i].String() < records[j].String() })
	}
	return zones, nil
}
func Digest(s domain.State) (string, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
