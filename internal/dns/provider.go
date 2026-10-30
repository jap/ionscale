package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/jsiebens/ionscale/internal/config"
	"github.com/libdns/azure"
	"github.com/libdns/cloudflare"
	"github.com/libdns/digitalocean"
	"github.com/libdns/googleclouddns"
	"github.com/libdns/libdns"
	"github.com/libdns/route53"
	"go.uber.org/zap"
)

var factories = map[string]func() libdns.RecordSetter{
	"azure":          azureProvider,
	"cloudflare":     cloudflareProvider,
	"digitalocean":   digitalOceanProvider,
	"googleclouddns": googleCloudDNSProvider,
	"route53":        route53Provider,
}

type Provider interface {
	SetRecord(ctx context.Context, recordType, recordName, value string) error
}

func NewProvider(config config.DNS) (Provider, error) {
	p := config.Provider
	if len(p.Zone) == 0 {
		return nil, nil
	}

	if p.Name == "" && p.PluginPath == "" {
		return nil, fmt.Errorf("invalid dns provider configuration, either name or plugin_path should be set")
	}

	if p.Name != "" && p.PluginPath != "" {
		return nil, fmt.Errorf("invalid dns provider configuration, only one of name or plugin_path should be set")
	}

	if !zoneMatchesSuffix(config.MagicDNSSuffix, p.Zone) {
		return nil, fmt.Errorf("invalid MagicDNS suffix [%s], not part of zone [%s]", config.MagicDNSSuffix, p.Zone)
	}

	if p.PluginPath != "" {
		return newPluginManager(p.PluginPath, fqdn(p.Zone), p.Configuration)
	}

	if factory, ok := factories[p.Name]; ok {
		return newProvider(p.Zone, p.Configuration, factory)
	}

	// Not compiled in: look for an out-of-process plugin in provider_dir.
	if pluginPath, err := resolvePluginPath(config.ProviderDir, p.Name); err != nil {
		return nil, err
	} else {
		return newPluginManager(pluginPath, fqdn(p.Zone), p.Configuration)
	}
}

// zoneMatchesSuffix reports whether suffix is the zone or lives inside it.
// strings.HasSuffix alone accepts "notexample.com" for zone "example.com".
func zoneMatchesSuffix(suffix, zone string) bool {
	s := strings.ToLower(strings.TrimSuffix(suffix, "."))
	z := strings.ToLower(strings.TrimSuffix(zone, "."))
	return s == z || strings.HasSuffix(s, "."+z)
}

// resolvePluginPath maps a provider name to <dir>/ionscale-<name>-dns, the
// naming convention of the ionscale/*-dns plugin repos.
func resolvePluginPath(dir, name string) (string, error) {
	builtins := make([]string, 0, len(factories))
	for k := range factories {
		builtins = append(builtins, k)
	}
	slices.Sort(builtins)

	if dir == "" {
		return "", fmt.Errorf("unknown dns provider %q (built-in providers: %s; to use an external plugin set dns.provider.plugin_path or dns.provider_dir)", name, strings.Join(builtins, ", "))
	}

	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return "", fmt.Errorf("invalid dns provider name %q", name)
	}

	candidate := filepath.Join(dir, "ionscale-"+name+"-dns")
	if runtime.GOOS == "windows" {
		candidate += ".exe"
	}

	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("dns provider %q not found: no plugin at %s (built-in providers: %s)", name, candidate, strings.Join(builtins, ", "))
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("dns plugin %s is not executable", candidate)
	}

	return candidate, nil
}

func newProvider(zone string, values json.RawMessage, factory func() libdns.RecordSetter) (Provider, error) {
	p := factory()
	if err := json.Unmarshal(values, p); err != nil {
		return nil, err
	}
	return &externalProvider{zone: fqdn(zone), setter: p}, nil
}

func azureProvider() libdns.RecordSetter {
	zap.L().Warn("Builtin azure DNS plugin is deprecated and will be removed in a future release.")
	return &azure.Provider{}
}

func cloudflareProvider() libdns.RecordSetter {
	zap.L().Warn("Builtin cloudflare DNS plugin is deprecated and will be removed in a future release.")
	return &cloudflare.Provider{}
}

func digitalOceanProvider() libdns.RecordSetter {
	zap.L().Warn("Builtin digitalocean DNS plugin is deprecated and will be removed in a future release.")
	return &digitalocean.Provider{}
}

func googleCloudDNSProvider() libdns.RecordSetter {
	zap.L().Warn("Builtin googleclouddns DNS plugin is deprecated and will be removed in a future release.")
	return &googleclouddns.Provider{}
}

func route53Provider() libdns.RecordSetter {
	zap.L().Warn("Builtin route53 DNS plugin is deprecated and will be removed in a future release.")
	return &route53.Provider{}
}

type externalProvider struct {
	zone   string
	setter libdns.RecordSetter
}

func (p *externalProvider) SetRecord(ctx context.Context, recordType, recordName, value string) error {
	_, err := p.setter.SetRecords(ctx, p.zone, []libdns.Record{{
		Type:  recordType,
		Name:  libdns.RelativeName(recordName, p.zone),
		Value: value,
		TTL:   1 * time.Minute,
	}})
	return err
}

func fqdn(v string) string {
	if strings.HasSuffix(v, ".") {
		return v
	}
	return fmt.Sprintf("%s.", v)
}
