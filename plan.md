# Plan: modern libdns provider support in ionscale (TransIP + anything libdns/Caddy supports)

## Goal

`ionscale` completes the ACME DNS-01 challenge (`tailscale cert`, `tailscale serve` with HTTPS) using **TransIP**, and structurally using **any** [libdns](https://github.com/libdns) provider — which is also Caddy's whole DNS-provider surface, since every `caddy-dns/*` module is a thin wrapper over a `libdns/*` package (e.g. `caddy-dns/transip` → `libdns/transip v1.1.2`).

## Decisions (Jasper, 2026-10-30)

1. **Fork first, upstream later.** Work happens in `jap/ionscale` (gh CLI is authenticated as `jap`); upstream changes are **prepared but never submitted without explicit per-PR consent** — upstream has had no commits since `280c626` (2025-05-28) anyway.
2. **Per-provider plugin repos**, matching the existing `ionscale/ionscale-<provider>-dns` pattern. No generic multi-provider runner.
3. *(mine)* **No breaking host changes.** The host keeps `libdns v0.2.3` + `libdns-plugin v0.1.0` and the deprecated built-ins keep working. New providers arrive out-of-process, one binary each. Rationale in "Why no libdns v1 host bump".
4. *(mine)* **Current ACME behaviour stays the default.** Cleanup, resolver and propagation tuning are opt-in flags with today's behaviour as the default value.

## Why no libdns v1 host bump (decision 3)

- The plugin wire is already version-independent: `proto/libdnsplugin.proto` (`Record{type,name,data,ttl}`), `Handshake` (MagicCookie + `ProtocolVersion: 1`) and `serve.go` are **byte-identical** between `libdns-plugin` v0.1.0 and v1.0.0; the v0.1.0→v1.0.0 diff is only `go.mod`, `go.sum` and ~10 lines of `grpc.go` (`libdns.Record{}` → `RR{}`).
- ⇒ A per-provider plugin built on ABI **v1.0.0** + `libdns/transip v1.1.2` already dispenses against the released ionscale v0.18.0 host. The official `ionscale/ionscale-cloudflare-dns@main` is already on that ABI (cloudflare v0.2.1 → libdns v1.1.0). Modern providers are reachable *without touching the host*.
- **Verified, from `Serve` source + a live round trip**: `Serve` wraps the provider in
  `configurableProvider`, whose `Configure` is `json.Unmarshal(message, &c.provider)` — the
  provider's own `Configure` is *not* called; the config lands on the value passed to `Serve`,
  which is why it must be a **pointer**.
- **Verified, in test output**: the transport is a **unix socket** (`network=unix`,
  `dispensed=*plugin.GRPCClient`), so TransIP credentials in `Configure` never ride TCP.
- Bumping the host would force deleting 4 of the 5 built-ins — `route53 v1.3.3`, `googleclouddns v1.1.0`, `azure v0.4.0`, `digitalocean` have **no** libdns-v1 release (their sources use `libdns.Record{Value:…}` and cannot compile against v1; only `cloudflare v0.2.x` migrated). That is a breaking config change (`name: route53` stops working) bought for nothing.
- `github.com/jsiebens/libdns-plugin` is frozen at v1.0.0 (2025). Risk of future wire drift is therefore low, and is pinned by a CI interop test (Phase 2), not by hoping.

## Caddy analogy, adapted

| Caddy | This plan |
|---|---|
| `caddy.RegisterModule` / module names in JSON | `dns.provider.name: transip` resolved against built-ins **or** a `plugin_dir` of `ionscale-<name>-dns` binaries |
| `xcaddy build --with github.com/…` | `ionscale plugin build transip` — fetches a per-provider repo and cross-compiles the plugin binary locally |
| one binary, many modules | deliberately **not** adopted: one repo + one binary per provider (decision 2) |
| module registry at caddyserver.com | the `ionscale/*` GitHub org listing + docs table (Phase 5), fork-published to `ghcr.io/jap/…` |

## Verified constraints (module proxy + source reads, 2026-10-30)

- Host pins: `internal/config/config.go:215-225` — `DNSProvider{Name, PluginPath, Zone, Configuration}`; `internal/dns/provider.go` — `name` XOR `plugin_path`, built-in `factories` map, `externalProvider.SetRecord` with hardcoded TTL 1m; `internal/dns/plugin.go` — `pluginManager.ensureRunning` restarts on every call with no backoff; `internal/handlers/dns.go` — `/machine/set-dns`, then `net.LookupTXT` poll every 5s up to 5 min, **returns 200 even on timeout** (silent failure); `internal/handlers/query_feature.go` — `serve` gated on `dnsProvider != nil && tailnet.DNSConfig.HttpsCertsEnabled`.
- `libdns/transip@v1.1.2` consumes generic `libdns.RR` (`client/client_dns.go: MarshallDNSRecords(*libdns.RR, zone)`), delegate to `pbergman/provider`; record-level control does delete-then-create per record; `expire` clamps to 60s–86400s (ionscale sends 60s ⇒ OK).
- Per-provider repo template (`ionscale/ionscale-cloudflare-dns`): `main.go` (3 lines), `go.mod`, `Dockerfile` (`FROM scratch`), `.goreleaser.yaml` (linux/darwin/windows × amd64/arm64, cosign, `ghcr.io/ionscale/<name>`, asset name `ionscale-<provider>-dns_<os>_<arch>`), `.github/workflows/release.yaml`. Latest release v0.1.0.
- Ecosystem is on libdns v1: caddy v2.11.7 → v1.1.1; transip v1.1.2 → v1.1.1; cloudflare v0.2.2, hetzner v1.0.0, godaddy v1.1.0, he v1.2.2, desec v1.1.1, ovh v1.1.0, rfc2136 v1.0.1, acmedns v0.5.0, powerdns, netcup v1.0.0.
- `Serve(&P{})` gotcha: config lands via `json.Unmarshal(msg, &c.provider)` on an interface field ⇒ **must pass a pointer** or credentials are silently dropped.

## Phase 0 — TransIP live, no fork required

- [x] New repo `jap/ionscale-transip-dns`, copied from the `ionscale-cloudflare-dns` template: `main.go` = `plugin.Serve(&transip.Provider{})` against `jsiebens/libdns-plugin v1.0.0` + `libdns/transip v1.1.2`
  - [x] pointer to `Serve`; `go mod tidy`; verify `go version -m` on the binary lists `transip v1.1.2` + `libdns v1.1.1`
  - [ ] `.goreleaser.yaml` → `project_name: ionscale-transip-dns`, images `ghcr.io/jap/ionscale-transip-dns`, cosign
- [x] Offline regression test: in-repo fake provider plugin (own module, ABI v1) + host-side go-plugin client asserting `SetRecords` gets `RR{Type:"TXT", Name:"_acme-challenge.<host>", TTL:1m}` and that `Configure` populated the struct — this test doubles as the ABI interop guard
- [ ] Deploy to the self-hosted ionscale at `is.bmw12.nl`: mount binary (or base the ionscale image on `ghcr.io/jap/ionscale-transip-dns` copy stage), `dns.provider: {plugin_path: /data/ionscale/plugins/ionscale-transip-dns, zone: bmw12.nl, config: {login, private_key, read_only: false, not_global_key, token_storage}}`
- [ ] TransIP preconditions documented + checked: API key from TransIP CP with `read_only:false` and TXT write rights, IP allowlist via `not_global_key`, tailnet base domain `bmw12.ion.bmw12.nl` must be **publicly authoritative** in TransIP for LE to validate, `expire` min 60s
- [x] Verify plugin transport is a Unix socket (go-plugin `AutoSecure`/`SecureConfig` defaults) — `Configure` carries the TransIP credentials
- [ ] Acceptance: `tailscale cert <node>.bmw12.ion.bmw12.nl` issues; TXT visible in TransIP CP during issuance
- [ ] Commit (new repo): `git commit -m "Phase 0: ionscale-transip-dns libdns plugin"`

## Phase 1 — Fork + additive host ergonomics (no config breaks)

- [x] Fork `jsiebens/ionscale` → `jap/ionscale`, add as `fork` remote, keep upstream `origin` for cherry-picking/PRs
- [x] `dns.provider_dir` config key (default empty): when set, `name` resolves to `<dir>/ionscale-<name>-dns`
  - [x] resolution order: explicit `plugin_path` → built-in `factories` → `provider_dir` lookup; existing configs unaffected
  - [x] executable-bit check with a clear error
- [x] Replace the `exec.Command("")` failure with `unknown dns provider %q (built-ins: …; plugin_dir: …)`
- [x] Fix `strings.HasSuffix(MagicDNSSuffix, Zone)` to a label-boundary check (`notexample.com` must not match zone `example.com`); only rejects configs that already failed at the API
- [ ] Keep `libdns v0.2.3` + `libdns-plugin v0.1.0` pins, with a comment stating the wire contract is `RR`-flat and compatible with plugin ABI v1
- [ ] `go build ./... && go vet ./...` + existing `tests/` integration suite green
- [ ] Commit: `git commit -m "Phase 1: provider_dir name resolution, provider lookup errors, zone validation"`

## Phase 2 — ABI interop contract as a test, not folklore

- [x] `tests/pluginabi/` (separate Go module, libdns v1 + plugin ABI v1): fake provider + tiny host client
  - [x] v1-built plugin dispensed by a host built on ABI v0.1.0 (the shipping combination) → `Configure` + `SetRecords` round-trip
  - [ ] v0.1.0-built plugin (download a released `ionscale-*-dns` v0.1.0 asset, or build a v0.1.0 fake) dispensed by an ABI-v1 host → proves both directions
  - [ ] assert failure modes: plugin killed mid-call → host restarts and retries; provider returns error → surfaces as the ACME error, not a 200
- [ ] Pin `libdns-plugin` in CI (dependency-review or a `// pinned for wire compat` guard test) so an unpinned ABI bump cannot silently change the contract
- [ ] Commit: `git commit -m "Phase 2: cross-version plugin ABI interop tests"`

## Phase 3 — xcaddy-style build tool

- [ ] `ionscale plugin build <provider>[@ref] [--os linux --arch arm64] [-o dir]`: clones/builds a per-provider plugin locally for architectures or unmerged providers not in a release
  - [ ] provider → repo map with an explicit `--repo <url>` override for names outside the org
  - [ ] cached workspace, `CGO_ENABLED=0`, prints + verifies checksums, cosign-verify when a signature exists
- [ ] `ionscale dns list-providers`: built-ins + binaries found in `plugin_dir` + known provider repos
- [ ] `ionscale dns check`: loads the configured provider, calls `GetRecords(zone)`/a TXT write-delete round trip, prints a pass/fail report (today there is no way to validate provider credentials short of triggering a real challenge)
- [ ] Commit: `git commit -m "Phase 3: plugin build tool + dns check"`

## Phase 4 — ACME-path correctness, all opt-in with today's behaviour as default

- [ ] `dns.provider.cleanup_after` (default `0` = off = current leak-until-overwrite behaviour): when >0, schedule `DeleteRecords` of `_acme-challenge` TXT that many seconds after a successful `Present` (Tailscale's `tailcfg.SetDNSRequest` carries no clean-up signal — flag it in the config docs as a deliberate workaround)
- [ ] `dns.provider.verify` (`public` | `provider` | `off`, default `public` = current `net.LookupTXT`) plus `verify_interval`/`verify_timeout` (defaults 5s/5min); `provider` mode reads back via `GetRecords` for zones that are not public
- [ ] Stop returning 200 on poll timeout: return an error (or log at error level + `dns.provider.verify_fail_open: true` to keep the old behaviour) so failures are visible instead of surfacing as an opaque `dnsproposed` from LE
- [ ] Error wrapping + credential masking (`private_key`, `api_token`, `client_secret`) in plugin/host logs
- [ ] Plugin restart backoff in `internal/dns/plugin.go` (currently restart-on-every-call)
- [ ] Tests: fake provider covering present → verify → cleanup, timeout paths, backoff
- [ ] Commit: `git commit -m "Phase 4: opt-in DNS-01 lifecycle correctness (cleanup, verify mode, backoff)"`

## Phase 5 — Provider catalogue, docs, upstream PRs

- [ ] Per-provider repo backlog, each a clone of the template with provider-specific README + example `config:` block, in demand order: `transip` (done in Phase 0), `hetzner`, `ovh`, `inwx`, `he`, `desec`, `godaddy`, `netcup`, `powerdns`, `rfc2136`, `ns1`, `gcore`, `route53` (v1 port is upstream's problem — pin whatever compiles)
  - [ ] note for each: does it need generic `RR` or does it type-switch on concrete record types? (transip: generic ⇒ compatible with the host's `RR`-only traffic; a provider that requires `libdns.TXT` structs would need the host to send typed records — a Phase-4-sized host change, flag per provider before adding)
- [ ] Rewrite `mkdocs/docs/configuration/dns-providers.md`: `plugin_dir` + `name` flow, per-provider install (download release asset / `ionscale plugin build`), TransIP section, "5 lines to add a provider" template, cosign verification, migration note
- [ ] Upstream PRs to `jsiebens/ionscale`: **never submitted without explicit, per-PR consent** (2026-10-30). Prepare branches + drafted PR text locally only. Slices, in order: (1) `provider_dir` + provider-lookup errors + zone validation, (2) ABI interop tests, (3) docs. Host libdns v1 bump + built-in removal only as a separate upstream proposal, if ever.
- [ ] `plan.md`/`TODO.md` reflect reality; release notes for the fork tag
- [ ] Commit: `git commit -m "Phase 5: provider catalogue, docs, upstream PR slices"`

## Rejected options

- **Generic multi-provider runner** (`config: {provider: transip, …}` selecting from a compiled-in registry): rejected by decision 2. Also needs a custom `ProviderPlugin{Impl: runner}` because stock `Serve` binds one provider at Serve time.
- **Host libdns v1 bump** as the first move: rejected as breaking (4 built-ins die) for no gain the wire doesn't already provide.
- **Delegated ACME DNS sidecar** (`adrg/acme-dns`, cert-manager-style webhook solvers): CNAME `_acme-challenge.<zone>` to a tiny authenticated DNS server, provider becomes a sidecar config with zero recompiles. Kept as the fallback if the fork turns out to be unmaintainable; adds a public moving part.
- **Reusing `caddy-dns/*` modules**: needs the `caddy/v2` module runtime; their `libdns/*` dependency is the useful half.
- **Runtime `.so` loading (`plugin.Open`)**: glibc/Go-version brittle, no version negotiation.

## Risks

- ABI drift if `libdns-plugin` ever changes the proto: bounded by the Phase 2 interop tests and pinned deps.
- Providers that type-switch on concrete libdns structs instead of accepting `RR`: the host only ever sends generic `RR`, so such a provider fails at runtime. Per-provider check required (Phase 5 note).
- TransIP specifics: record-level control is delete-then-create (concurrent challenges on the same name could race — ACME TXT names are per-node unique, so acceptable); token storage is file-backed at `os.TempDir()/transip` (multi-replica needs shared or memory storage).
- ionscale returning 200 on verification timeout hides real failures until Phase 4; TransIP propagation must be assumed < 5 min in the meantime.
