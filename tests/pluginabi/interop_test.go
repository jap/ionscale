// Package pluginabi pins the wire contract between ionscale and its DNS
// provider plugins.
//
// ionscale builds against github.com/jsiebens/libdns-plugin v0.1.0, whose
// records are the libdns v0.2.3 struct (Record{Value}). Modern providers such
// as libdns/transip only exist for libdns v1, where Record is an interface and
// the field is RR{Data}. Those two worlds talk to each other because the gRPC
// contract is unchanged: proto/libdnsplugin.proto, plugin.go (handshake,
// dispense name), client.go and serve.go are byte-identical between
// libdns-plugin v0.1.0 and v1.0.0 — only the in-process record type differs.
//
// This test is the regression guard for that claim. ./fakeplugin is a separate
// Go module pinned to libdns v1 and plugin ABI v1; the host below is the
// shipping ionscale build (ABI v0.1.0). If the two ever stop interlocking,
// this fails here instead of during a live ACME issuance.
package pluginabi_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	dnsplugin "github.com/jsiebens/libdns-plugin"
	"github.com/libdns/libdns"
)

// TestHostV0DispensesPluginV1 covers the combination that ships: ionscale built
// on plugin ABI v0.1.0 dispensing a provider written against libdns v1.
func TestHostV0DispensesPluginV1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bin := filepath.Join(t.TempDir(), "fakeplugin")
	build := exec.CommandContext(ctx, "go", "build", "-C", "./fakeplugin", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build v1 fake plugin: %v\n%s", err, out)
	}

	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(), "FAKE_ECHO_CONFIG=1")

	client, provider, err := dnsplugin.NewClient(cmd)
	if err != nil {
		t.Fatalf("host (abi v0.1.0) dispense plugin (abi v1): %v", err)
	}
	defer client.Kill()

	// The config blob must land on the provider, otherwise credentials are
	// silently dropped and the only symptom is an authentication failure at the
	// DNS provider during certificate issuance.
	if err := provider.Configure(ctx, json.RawMessage(`{"login":"account","read_only":false}`)); err != nil {
		t.Fatalf("configure across abis: %v", err)
	}

	back, err := provider.GetRecords(ctx, "example.com.")
	if err != nil {
		t.Fatalf("get records across abis: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("want 1 echoed config record, got %d", len(back))
	}
	if want := "login=account,read_only=false"; back[0].Value != want {
		t.Fatalf("config did not cross the abi boundary:\nwant %q\ngot  %q", want, back[0].Value)
	}

	// A challenge record written with the host's v0.2.3 Record struct must
	// arrive as a libdns v1 RR with the same fields, and come back intact.
	zone := "example.com."
	recs := []libdns.Record{{
		Type:  "TXT",
		Name:  libdns.RelativeName("_acme-challenge.home.example.com", zone),
		Value: "dGVzdC10b2tlbg",
		TTL:   time.Minute,
	}}

	if _, err := provider.SetRecords(ctx, zone, recs); err != nil {
		t.Fatalf("set records across abis: %v", err)
	}

	back, err = provider.GetRecords(ctx, zone)
	if err != nil {
		t.Fatalf("get records after set: %v", err)
	}
	if len(back) != 2 {
		t.Fatalf("want config echo + challenge record, got %d", len(back))
	}

	got := back[1]
	if got.Name != "_acme-challenge.home" {
		t.Errorf("name = %q, want %q", got.Name, "_acme-challenge.home")
	}
	if got.Type != "TXT" {
		t.Errorf("type = %q, want TXT", got.Type)
	}
	if got.Value != "dGVzdC10b2tlbg" {
		t.Errorf("value = %q, want %q", got.Value, "dGVzdC10b2tlbg")
	}
	if got.TTL != time.Minute {
		t.Errorf("ttl = %v, want 1m", got.TTL)
	}

	if _, err := provider.DeleteRecords(ctx, zone, recs); err != nil {
		t.Fatalf("delete records across abis: %v", err)
	}
	if back, err := provider.GetRecords(ctx, zone); err != nil {
		t.Fatalf("get records after delete: %v", err)
	} else if len(back) != 1 {
		t.Fatalf("want only the config echo left, got %d", len(back))
	}
}
