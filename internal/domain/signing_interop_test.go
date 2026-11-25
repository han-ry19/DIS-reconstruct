package domain

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestJavaScriptSigningBytes(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js required for cross-language signing test")
	}
	cases := []Request{
		{Domain: "test", Operation: "data.register", Actor: "alice.uid", RequestID: "vector", Timestamp: 1800000000,
			Parameters: Parameters{DataID: "item.data", Address: "https://example.org/?a=1&b=<two>", Profile: &DataProfile{Label: "中文 😀", Description: "line\n\t\"\\\u2028\u2029"}}},
		{Domain: "test", Operation: "authorization.request", Actor: "app.uid", RequestID: "empty", Timestamp: 1800000000, Parameters: Parameters{Permissions: []string{}, Profile: &DataProfile{}}},
		{Domain: "test", Operation: "authorization.update", Actor: "alice.uid", RequestID: "perms", Timestamp: 1800000000, Parameters: Parameters{Field: "permissions", NewContent: "[\"Read\",\"Update\"]", ExpiresAt: 1800003600, Permissions: []string{"Read", "Update"}}},
	}
	for _, r := range cases {
		input, _ := json.Marshal(r)
		script := "const fs=require('node:fs');const s=require('../../benchmarks/caliper/signing');const r=JSON.parse(fs.readFileSync(0,'utf8'));r.parameters=Object.fromEntries(Object.entries(r.parameters).reverse());process.stdout.write(s.signingBytes(r));"
		cmd := exec.Command("node", "-e", script)
		cmd.Stdin = bytes.NewReader(input)
		actual, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node: %v: %s", err, actual)
		}
		expected, err := SigningBytes(r)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("signing bytes differ:\nGo: %s\nJS: %s", expected, actual)
		}
	}
}
