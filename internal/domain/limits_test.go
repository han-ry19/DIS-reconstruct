package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestOversizedParametersLeaveStateUnchanged(t *testing.T) {
	cases := []struct {
		name, operation string
		parameters      Parameters
	}{
		{"registration address", "data.register", Parameters{DataID: "new.data", Address: "https://example.org/" + strings.Repeat("x", 70000), DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{}}},
		{"updated address", "data.update", Parameters{DataID: "item.data", Field: "URI", NewContent: "https://example.org/" + strings.Repeat("x", 70000)}},
		{"profile description", "data.register", Parameters{DataID: "new.data", Address: "https://example.org", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{Description: strings.Repeat("x", MaxProfileDescriptionBytes+1)}}},
		{"profile UTF8 bytes", "data.register", Parameters{DataID: "new.data", Address: "https://example.org", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{Label: strings.Repeat("界", 100)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, keys := updateFixture()
			before, _ := json.Marshal(s)
			r := signedUpdate(t, keys, tc.operation, "alice.uid", "oversized", tc.parameters)
			if _, err := Apply(&s, r, updateNow); !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected invalid request, got %v", err)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejected request changed state")
			}
		})
	}
}

func TestAddressLimitAppliesToRegistrationAndUpdate(t *testing.T) {
	for _, operation := range []string{"data.register", "data.update"} {
		s, keys := updateFixture()
		address := "https://example.org/" + strings.Repeat("x", MaxAddressBytes-len("https://example.org/"))
		p := Parameters{DataID: "limit.data", Address: address, DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{}}
		if operation == "data.update" {
			p = Parameters{DataID: "item.data", Field: "URI", NewContent: address}
		}
		r := signedUpdate(t, keys, operation, "alice.uid", "at-limit", p)
		if _, err := Apply(&s, r, updateNow); err != nil {
			t.Fatal(err)
		}
		if operation == "data.update" {
			p.NewContent += "x"
		} else {
			p.DataID = "over.data"
			p.Address += "x"
		}
		r = signedUpdate(t, keys, operation, "alice.uid", "over-limit", p)
		if _, err := Apply(&s, r, updateNow); !errors.Is(err, ErrInvalid) {
			t.Fatalf("over limit accepted: %v", err)
		}
	}
}
