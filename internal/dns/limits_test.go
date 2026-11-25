package dns

import (
	"github.com/han-ry19/DIS-reconstruct/internal/domain"
	mdns "github.com/miekg/dns"
	"strings"
	"testing"
)

func TestMaximumAddressFitsDNSMessages(t *testing.T) {
	s := fixture()
	for id, d := range s.Data {
		d.Address = "https://example.org/" + strings.Repeat("x", domain.MaxAddressBytes-len("https://example.org/"))
		s.Data[id] = d
	}
	zones, err := Project(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, records := range zones {
		for _, record := range records {
			if mdns.Len(record) > 48000 {
				t.Fatal("record exceeds AXFR batch limit")
			}
			msg := new(mdns.Msg)
			msg.Answer = []mdns.RR{record}
			if _, err := msg.Pack(); err != nil {
				t.Fatalf("%T length=%d: %v", record, mdns.Len(record), err)
			}
		}
	}
}
