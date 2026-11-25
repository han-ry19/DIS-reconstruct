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
		{"registration address", "data.register", Parameters{DataID: "3fe16f43-7f15-460f-b5da-06b45df2df3e.data", Address: "https://example.org/" + strings.Repeat("x", 70000), DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{}}},
		{"updated address", "data.update", Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "URI", NewContent: "https://example.org/" + strings.Repeat("x", 70000)}},
		{"profile description", "data.register", Parameters{DataID: "3fe16f43-7f15-460f-b5da-06b45df2df3e.data", Address: "https://example.org", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{Description: strings.Repeat("x", MaxProfileDescriptionBytes+1)}}},
		{"profile UTF8 bytes", "data.register", Parameters{DataID: "3fe16f43-7f15-460f-b5da-06b45df2df3e.data", Address: "https://example.org", DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{Label: strings.Repeat("界", 100)}}},
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
		p := Parameters{DataID: "dfa67d9a-6d96-470f-ac5f-11eb70398aa1.data", Address: address, DataSignature: base64.StdEncoding.EncodeToString(make([]byte, 64)), Profile: &DataProfile{}}
		if operation == "data.update" {
			p = Parameters{DataID: "e50021a0-bd86-4273-9cfe-c0ffeed3dcaf.data", Field: "URI", NewContent: address}
		}
		r := signedUpdate(t, keys, operation, "alice.uid", "at-limit", p)
		if _, err := Apply(&s, r, updateNow); err != nil {
			t.Fatal(err)
		}
		if operation == "data.update" {
			p.NewContent += "x"
		} else {
			p.DataID = "be6939d6-100c-4783-bed0-8d944eef64e3.data"
			p.Address += "x"
		}
		r = signedUpdate(t, keys, operation, "alice.uid", "over-limit", p)
		if _, err := Apply(&s, r, updateNow); !errors.Is(err, ErrInvalid) {
			t.Fatalf("over limit accepted: %v", err)
		}
	}
}
