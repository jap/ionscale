# Configuring DNS providers

ionscale supports integration with various DNS providers to enable Tailscale's HTTPS certificate functionality. When a DNS provider is properly configured, ionscale can automatically manage TXT records required for the DNS-01 challenge when requesting certificates.

## Why configure a DNS provider

While not strictly required for basic ionscale operation, configuring a DNS provider enables important Tailscale features:

1. **Tailscale HTTPS Certificates**: Allows nodes to receive valid HTTPS certificates for their Tailscale hostnames, enabling secure web services within your tailnet.
2. **Tailscale Serve**: Supports the `tailscale serve` feature, which allows users to easily share web services with proper HTTPS.

Without a configured DNS provider, these features will not be available to your users.

## Supported DNS providers

ionscale supports DNS providers through two methods:

### Built-in providers (deprecated)

ionscale includes built-in support for the following DNS providers using [libdns](https://github.com/libdns) libraries:

- [Azure DNS](https://github.com/libdns/azure)
- [Cloudflare](https://github.com/libdns/cloudflare)
- [DigitalOcean](https://github.com/libdns/digitalocean)
- [Google Cloud DNS](https://github.com/libdns/googleclouddns)
- [Amazon Route 53](https://github.com/libdns/route53)

!!! warning "Built-in providers are deprecated"
    The built-in DNS providers are deprecated and will be removed in a future release. Please migrate to external DNS plugins for continued support.

### External DNS plugins (recommended)

ionscale now supports external DNS plugins through a plugin system. This allows for:

- **Extensibility**: Add support for any DNS provider without modifying ionscale
- **Maintainability**: Plugins are maintained independently
- **Flexibility**: Plugin configuration specific to each provider's needs

!!! info "Plugin availability"
    External DNS plugins implement the [libdns-plugin](https://github.com/libdns/libdns-plugin) interface. Official plugin implementations can be found in the [ionscale GitHub organization](https://github.com/ionscale) with repositories named `ionscale-<provider>-dns`. You can also create your own following the plugin specification.

## DNS provider configuration

To configure a DNS provider, add the appropriate settings to your ionscale configuration file (`config.yaml`):

### Built-in provider configuration

```yaml
dns:
  # The base domain for MagicDNS FQDN hostnames
  magic_dns_suffix: "ionscale.net"
  
  # DNS provider configuration for HTTPS certificates
  provider:
    # Name of your DNS provider
    name: "cloudflare"
    # The DNS zone to use (typically your domain name)
    zone: "example.com"
    # Provider-specific configuration (varies by provider)
    config:
      # Provider-specific credentials and settings go here
      # See provider-specific examples below
```

### External plugin configuration

A plugin runs as its own binary. Point ionscale at it with an explicit path:

```yaml
dns:
  # The base domain for MagicDNS FQDN hostnames
  magic_dns_suffix: "ionscale.net"
  
  # DNS provider configuration for HTTPS certificates
  provider:
    # Path to your DNS plugin executable
    plugin_path: "/path/to/your/dns-plugin"
    # The DNS zone to use (typically your domain name)
    zone: "example.com"
    # Plugin-specific configuration (varies by plugin)
    config:
      # Plugin-specific credentials and settings go here
      # See plugin documentation for configuration options
```

### Provider directory (recommended)

Instead of a full path, set one `provider_dir` and select plugins by `name`. ionscale resolves
`name: <provider>` to `<provider_dir>/ionscale-<provider>-dns`, the naming convention of the
`ionscale/*-dns` plugin repos:

```yaml
dns:
  magic_dns_suffix: "ionscale.net"
  provider_dir: "/var/lib/ionscale/plugins"
  provider:
    name: "transip"     # resolves to /var/lib/ionscale/plugins/ionscale-transip-dns
    zone: "example.com"
    config:
      login: "your-account"
      private_key: "/etc/ionscale/private_key.pem"
```

Resolution order when `provider.name` is set:

1. A built-in compiled-in provider (`azure`, `cloudflare`, `digitalocean`, `googleclouddns`, `route53`).
2. Otherwise, `provider_dir` lookup for `ionscale-<name>-dns`.

`name` and `plugin_path` are mutually exclusive: use `plugin_path` for a binary at an arbitrary path,
`name` for a built-in or a plugin found in `provider_dir`. The plugin binary must be executable
(`chmod 755`); otherwise ionscale reports `dns plugin <path> is not executable`.

!!! info "`provider_dir` is fork-only until released"
    Released ionscale ignores `provider_dir`. On a released build, use `dns.provider.plugin_path` to point
    at the plugin binary directly.

### Important requirements

1. The `magic_dns_suffix` must be the zone itself or a subdomain of it, matched on label boundaries: with zone
   `example.com`, suffix `ts.example.com` is accepted but `notexample.com` is rejected
   (`invalid MagicDNS suffix [ts.notexample.com], not part of zone [example.com]`). The suffix does not need to
   be a delegated subdomain — it may be plain names inside the zone, as long as it is publicly authoritative there.
2. MagicDNS must be enabled for the tailnet to use HTTPS certificates.
3. You must have administrative access to the DNS zone to configure the provider.
4. For external plugins, the plugin executable must be accessible and executable by the ionscale process.

## Provider-specific examples

### TransIP

No official plugin exists for TransIP; use [`jap/ionscale-transip-dns`](https://github.com/jap/ionscale-transip-dns),
which wraps [`libdns/transip`](https://github.com/libdns/transip).

```yaml
dns:
  magic_dns_suffix: "ion.example.com"
  provider_dir: "/var/lib/ionscale/plugins"
  provider:
    name: "transip"    # or: plugin_path: /var/lib/ionscale/plugins/ionscale-transip-dns
    zone: "example.com"
    config:
      login: "your-transip-account"
      private_key: "/etc/ionscale/transip_private_key.pem"   # RSA PKCS#8 PEM, inline or a file path
      read_only: false
      not_global_key: false
      expiration_time: "1 week"
      token_storage: "/var/lib/ionscale/transip-tokens"
```

Auth keys belong to `libdns/transip`. There is no `username` or `api_key_path` key — those are silently
ignored. TransIP API keys are **RSA** (PKCS#8 PEM), generated at <https://www.transip.nl/cp/account/api>.
`read_only` must be `false` or the API token cannot write TXT records. `token_storage` defaults to a temp
directory; a fixed path avoids a fresh auth token on every restart.

### Cloudflare

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    name: "cloudflare"
    zone: "example.com"
    config:
      api_token: "your-cloudflare-api-token"
```

For Cloudflare, create an API token with the "Edit" permission for "Zone:DNS".

### Azure DNS

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    name: "azure"
    zone: "example.com"
    config:
      tenant_id: "your-tenant-id"
      client_id: "your-client-id"
      client_secret: "your-client-secret"
      subscription_id: "your-subscription-id"
      resource_group_name: "your-resource-group"
```

For Azure, create a service principal with "DNS Zone Contributor" role for your DNS zone's resource group.

### Amazon Route 53

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    name: "route53"
    zone: "example.com"
    config:
      access_key_id: "your-access-key-id"
      secret_access_key: "your-secret-access-key"
      # Optional region, defaults to us-east-1
      region: "us-east-1"
      ...
```

For AWS Route 53, create an IAM user with permissions to modify records in your hosted zone.

### Google Cloud DNS

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    name: "googleclouddns"
    zone: "example-com" # Note: GCP uses zone names without dots
    config:
      gcp_project: "your-project-id"
      # Optional path to a service account key file
      # gcp_application_default: "/path/to/service-account-key.json"
```

For Google Cloud DNS, create a service account with the "DNS Administrator" role.

### DigitalOcean

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    name: "digitalocean"
    zone: "example.com"
    config:
      auth_token: "your-digitalocean-api-token"
```

For DigitalOcean, create an API token with read and write access.

## External DNS plugin examples

### Using a hypothetical Cloudflare plugin

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    plugin_path: "/usr/local/bin/libdns-cloudflare-plugin"
    zone: "example.com"
    config:
      api_token: "your-cloudflare-api-token"
```

### Using a custom DNS plugin

```yaml
dns:
  magic_dns_suffix: "ts.example.com"
  provider:
    plugin_path: "/opt/dns-plugins/my-custom-provider"
    zone: "example.com"
    config:
      # Configuration specific to your custom plugin
      endpoint: "https://api.mydnsprovider.com"
      api_key: "your-api-key"
      custom_setting: "value"
```

!!! tip "Plugin development"
    To create your own DNS plugin, implement the [libdns-plugin](https://github.com/libdns/libdns-plugin) interface. The plugin system uses HashiCorp's go-plugin framework for communication between ionscale and your plugin.

## Enabling HTTPS certificates for a tailnet

After configuring a DNS provider in your ionscale server, you can enable HTTPS certificates for a tailnet:

```bash
# Enable MagicDNS and HTTPS certificates for a tailnet
ionscale tailnets set-dns --tailnet "my-tailnet" --magic-dns --https-certs=true
```

!!! note "Restart clients to pick up the capability"
    ionscale adds the `https` capability to a node's netmap only when HTTPS certificates are enabled for the
    tailnet. Clients read it at map time, so a device already running (for example a Home Assistant Tailscale
    add-on that gates `serve` on `Self.CapMap.https`) must be restarted to see the new capability after this
    command.

## Verifying DNS provider configuration

To verify that your DNS provider is correctly configured:

1. Configure a tailnet with MagicDNS and HTTPS certificates enabled.
2. Connect a device to the tailnet.
3. On the device, run:
   ```bash
   tailscale cert <hostname>
   ```
4. If successful, the command will obtain a certificate for the hostname.
5. You should see TXT records created in your DNS zone for the ACME challenge.

## Troubleshooting

If you encounter issues with DNS provider integration:

### General troubleshooting

1. **Check DNS Provider Credentials**: Ensure the credentials in your configuration have sufficient permissions.
2. **Verify Zone Configuration**: Make sure the zone in your configuration matches your actual DNS zone.
3. **Check MagicDNS Settings**: Confirm that `magic_dns_suffix` is properly configured as a subdomain of your zone.
4. **Review Server Logs**: The ionscale server logs may contain error messages related to DNS provider integration.
5. **Test DNS Record Creation**: Some providers offer tools to test API access for creating and updating DNS records.

### External plugin troubleshooting

1. **Plugin Executable**: Ensure the plugin path is correct and the executable has proper permissions.
2. **Plugin Logs**: Check both ionscale logs and any plugin-specific logs for error messages.
3. **Plugin Health**: ionscale automatically restarts failed plugins, but persistent failures may indicate configuration issues.
4. **Unknown provider**: `unknown dns provider %q` means the name is neither a built-in nor found in
   `provider_dir`. Set `dns.provider_dir` to the directory holding `ionscale-<name>-dns`, or use `plugin_path`.

### Home Assistant behind `tailscale serve`

When the tailnet node is a Home Assistant Tailscale add-on, `serve` fails (the add-on's `share-homeassistant`
service exits and tears down its container, and the browser shows `400: Bad Request`) unless Home Assistant
itself is configured as behind a reverse proxy. This is a Home Assistant setting, not an ionscale or DNS-provider
problem: Home Assistant's `forwarded_middleware` returns `400 Bad Request` for the `X-Forwarded-For` header that
`tailscale serve` adds.

Set the following in Home Assistant's `configuration.yaml` (or Settings > System > Network > HTTP server >
Reverse proxy) and then **restart Home Assistant Core** — an add-on restart alone does not reload it:

```yaml
http:
  use_x_forwarded_for: true
  trusted_proxies:
    - 127.0.0.1
```

Verify before restarting the add-on:

```bash
curl -s -o /dev/null -w'%{http_code}\n' -H 'X-Forwarded-For: 127.0.0.1' http://127.0.0.1:8123/
```

`200` means serve will work; `400` means Home Assistant still does not trust the proxy. Keep
`share_homeassistant: serve` — `funnel` is not supported by ionscale.

### Notes on serve and renames

- **`tailscale serve status` shows nothing**: expected for the Home Assistant add-on, which runs serve in the
  foreground (`--bg=false`). Foreground configs are not reported by `serve status` and vanish when the CLI exits.
  Use `tailscale serve status --json` and the add-on logs instead.
- **Renamed nodes**: renaming a node in ionscale pins the name server-side and regenerates its cert domains. The
  node's old name no longer has a valid cert domain, and stale `_acme-challenge.<old-name>` TXT records remain in
  the zone — ionscale only ever writes (`SetRecords`), never deletes.

