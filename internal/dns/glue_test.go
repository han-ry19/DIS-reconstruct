package dns

import (
	mdns "github.com/miekg/dns"
	"testing"
)

func TestConfiguredNameserverGlue(t *testing.T) {
	s := fixture()
	for _, address := range []string{"192.0.2.53", "198.51.100.1"} {
		zones, err := Project(s, address)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"uid.", "data."} {
			found := false
			for _, rr := range zones[name] {
				if a, ok := rr.(*mdns.A); ok && a.Hdr.Name == "ns."+name {
					found = true
					if a.A.String() != address {
						t.Fatal(a)
					}
				}
			}
			if !found {
				t.Fatal("missing glue", name)
			}
		}
		rows, err := encodeRecords(s, address)
		if err != nil {
			t.Fatal(err)
		}
		if rows["uid."]["ns"] == "" {
			t.Fatal("Redis glue missing")
		}
	}
	for _, bad := range []string{"", "hostname", "::1"} {
		if _, err := Project(s, bad); err == nil {
			t.Fatal("invalid glue accepted", bad)
		}
	}
}
