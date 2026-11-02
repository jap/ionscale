# TODO

Decisions recorded 2026-10-30 (see `plan.md`): fork `jap/ionscale` and upstream later; per-provider plugin repos; host stays on `libdns v0.2.3` + plugin ABI v0.1.0 (no breaking config); ACME behaviour stays the default with opt-in correctness flags.

Open now:

- [ ] Upstream PRs to `jsiebens/ionscale`: **blocked pending explicit consent** — nothing gets submitted until approved per PR. Prepare branches + drafted PR text only.
- [ ] `jap/ionscale-transip-dns` is **private**; flip to public when the README/licence are settled
- [ ] No `LICENSE` file in `jap/ionscale-transip-dns` — holder/choice undecided (upstream `ionscale/*-dns` are BSD-3-Clause); binaries are unpublished until then
- [ ] Nothing released: `jap/ionscale-transip-dns` has no tag, so no release assets or ghcr images exist yet; the test machine needs the binary copied from `/tmp/pluginbuild/` (or tag `v0.1.0` to let goreleaser build it)
- [ ] Fork work is on branch `phase1-provider-dir` (pushed); `main` on the fork still equals upstream `280c626`
- [ ] TransIP API credentials for the live acceptance test on `bmw12.nl`: `login` + an **RSA** private key (PKCS#8 PEM — `x509.ParsePKCS8PrivateKey`/`*rsa.PrivateKey` in `libdns/transip/provider_config.go`, TransIP does not do ed25519), `read_only:false`, IP allowlist via `not_global_key`.
- [ ] Unknown whether the deployed ionscale at `is.bmw12.nl` runs as a container with a writable mount for `/data/ionscale/plugins/` — determines the Phase 0 deployment shape (binary mount vs custom image stage).
- [ ] Per provider: confirm it accepts generic `libdns.RR`. The host only ever sends `RR{type,name,data,ttl}`; a provider that type-switches on concrete structs (`libdns.TXT`, `libdns.Address`) fails at runtime. transip verified generic; unverified for the rest of the Phase 5 catalogue.

Unverified claims that must be observed before being trusted:

- [~] v1-built plugin dispensed by an ABI-v0.1.0 host: **done and passing** (`tests/pluginabi`, config blob + TXT challenge record round-trip). Still open: the reverse direction, a v0.1.0-built plugin against an ABI-v1 host — irrelevant until ionscale itself moves to v1.
- [x] RESOLVED 2026-10-30: transport is a unix socket (`network=unix`, `dispensed=*plugin.GRPCClient`); `ProviderPlugin` embeds `NetRPCUnsupportedPlugin`, so gRPC only.
- [ ] TransIP live zone test not run. Token storage is file-backed at `os.TempDir()/transip`; behaviour under multiple ionscale replicas unverified.
- [ ] `_acme-challenge` TXT records are never deleted today (no clean-up signal in `tailcfg.SetDNSRequest`) and `internal/handlers/dns.go` returns HTTP 200 even when the 5-minute public-DNS poll times out. Consequences for stale records / LE rate limits not measured.

## Open from 2026-11-02 (cert issuance verified on `pi`)
- [ ] HA add-on still refuses HTTPS: waiting on `tailscale status --self --peers=false --json | jq '.Self.CapMap'` output from the HA node and the add-on log around `share-homeassistant`. Gate is `CapMap.https` ← `ionscale tailnets set-dns --https-certs=true`; needs an add-on restart to pick up the new netmap.
- [ ] Stale `_acme-challenge.*` TXT records accumulate in `bmw12.nl`: nothing deletes them (ionscale has no cleanup; Phase 4 `cleanup_after`). Sweep in TransIP CP after the HA cert works.
- [ ] ionscale's `/machine/set-dns` returns 200 after a 5-minute poll timeout even when the record never resolved publicly (`internal/handlers/dns.go:58-63`): this turned a config question into a 5-minute stall with no error. Phase 4 makes it opt-in visible (`verify_fail_open`).
- [ ] ionscale polls public visibility with **the server's own resolver** (`net.LookupTXT`); a split-horizon resolver on `is.bmw12.nl` would make every issuance fail after 5 minutes. Not yet checked on that host: `resolvectl status`, `dig +short TXT _acme-challenge.test.bmw12.nl`.
- [ ] No tag on `jap/ionscale-transip-dns` → no release assets; arm64 binary was copied by hand from `/tmp/pluginbuild/`. Blocked on the licence decision.
- [ ] Phase 1's existing integration suite (`tests/`, dockertest) has not been run against the fork branch yet — needs Docker and an image build.
