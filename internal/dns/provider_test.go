package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestZoneMatchesSuffix covers the label-boundary rule: the MagicDNS suffix
// must be the zone itself or a subdomain of it. The previous strings.HasSuffix
// check accepted "notexample.com" for zone "example.com", which let ionscale
// start and then fail at the provider API for every challenge.
func TestZoneMatchesSuffix(t *testing.T) {
	cases := []struct {
		suffix string
		zone   string
		want   bool
	}{
		{"example.com", "example.com", true},
		{"example.com.", "example.com", true},
		{"ion.example.com", "example.com", true},
		{"ion.EXAMPLE.com.", "example.com", true},
		{"a.b.example.com", "example.com", true},
		{"notexample.com", "example.com", false},
		{"example.com.evil.net", "example.com", false},
		{"other.zone", "example.com", false},
		{"", "example.com", false},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s", c.suffix, c.zone), func(t *testing.T) {
			if got := zoneMatchesSuffix(c.suffix, c.zone); got != c.want {
				t.Errorf("zoneMatchesSuffix(%q, %q) = %t, want %t", c.suffix, c.zone, got, c.want)
			}
		})
	}
}

// TestResolvePluginPath covers the name -> binary lookup introduced by
// dns.provider_dir: the ionscale/*-dns naming convention, the executable-bit
// requirement, and the error that replaces the old `exec.Command("")`
// "fork/exec : no such file or directory" for an unknown provider name.
func TestResolvePluginPath(t *testing.T) {
	dir := t.TempDir()

	full := filepath.Join(dir, "ionscale-transip-dns")
	if err := os.WriteFile(full, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ionscale-notexecutable-dns"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePluginPath(dir, "transip")
	if err != nil {
		t.Fatalf("resolve transip: %v", err)
	}
	if got != full {
		t.Errorf("resolved %q, want %q", got, full)
	}

	if _, err := resolvePluginPath(dir, "notexecutable"); err == nil {
		t.Error("want an error for a plugin without the executable bit")
	}

	if _, err := resolvePluginPath(dir, "nothere"); err == nil {
		t.Error("want an error for a missing plugin binary")
	}

	// Without provider_dir an unknown name must report the supported names
	// instead of trying to start an empty path.
	_, err = resolvePluginPath("", "transip")
	if err == nil {
		t.Fatal("want an error when provider_dir is unset")
	}
	for _, want := range []string{"transip", "cloudflare", "route53", "plugin_path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err.Error(), want)
		}
	}

	for _, name := range []string{"", ".", "..", "a/b", `a\b`} {
		if _, err := resolvePluginPath(dir, name); err == nil {
			t.Errorf("want an error for provider name %q", name)
		}
	}
}
