package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	plugin "github.com/jsiebens/libdns-plugin"
	"github.com/libdns/libdns"
)

// fake speaks plugin ABI v1 (libdns v1 RR) and keeps records in memory.
//
// It exists so ionscale, which is built against plugin ABI v0.1.0 (libdns
// v0.2.3), can dispense a plugin written against the modern libdns API: the
// combined test in the parent directory proves the wire contract holds across
// those two versions. This module is intentionally a separate Go module so the
// two ABIs can coexist in one repository.
type fake struct {
	Login    string `json:"login"`
	ReadOnly bool   `json:"read_only"`

	mu      sync.Mutex
	records []libdns.Record
}

// GetRecords echoes the decoded config as a TXT record, so the host can prove
// its dns.provider.config blob actually landed. Serve wraps the provider in a
// configurableProvider that json.Unmarshals straight into the value given to
// Serve, which is why main passes a pointer.
func (f *fake) GetRecords(_ context.Context, zone string) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]libdns.Record, 0, len(f.records)+1)
	if os.Getenv("FAKE_ECHO_CONFIG") != "" {
		out = append(out, libdns.RR{
			Name: libdns.RelativeName("_fake-config", zone),
			Type: "TXT",
			Data: fmt.Sprintf("login=%s,read_only=%t", f.Login, f.ReadOnly),
		})
	}
	out = append(out, f.records...)
	return out, nil
}

func (f *fake) AppendRecords(_ context.Context, _ string, recs []libdns.Record) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, recs...)
	return recs, nil
}

func (f *fake) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	if _, err := f.DeleteRecords(ctx, zone, recs); err != nil {
		return nil, err
	}
	return f.AppendRecords(ctx, zone, recs)
}

func (f *fake) DeleteRecords(_ context.Context, _ string, recs []libdns.Record) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var kept []libdns.Record
	for _, have := range f.records {
		drop := false
		for _, want := range recs {
			if have.RR().Name == want.RR().Name && have.RR().Type == want.RR().Type && have.RR().Data == want.RR().Data {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, have)
		}
	}
	f.records = kept
	return recs, nil
}

func main() {
	plugin.Serve(&fake{})
}
