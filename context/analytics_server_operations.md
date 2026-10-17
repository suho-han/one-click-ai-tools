# Analytics Server Operations

Deployed 2026-10-14. The collector binary is `cmd/oct-analytics-server`
(public ingest + tailnet-private dashboard, see `internal/analyticsserver`).
Status: live and receiving at `https://oct.suhohan.kr`.

## Topology

```
release binaries ──POST /v1/events,/v1/feedback──▶ Cloudflare DNS (A record, DNS only)
                                                      │ oct.suhohan.kr = 35.252.226.40
                                                      ▼
                        Caddy (caddy.service, Let's Encrypt, ports 80/443) on gcp-free-ops
                                                      │ reverse_proxy
                                                      ▼
                              oct-analytics.service  127.0.0.1:8843 (ingest)
                                                     127.0.0.1:8844 (dashboard, NOT routed)
```

- Host: `gcp-free-ops` (Tailscale alias; Ubuntu 24.04 x86_64), GCP project
  `gen-lang-client-0084410224`, zone `us-west1-b`.
- Ingest is the only publicly routed listener. The dashboard has no public
  route (Caddy proxies only to 8843; the catch-all on the collector returns
  404 for unknown paths); access it over Tailscale SSH.
- Binary: `/usr/local/bin/oct-analytics-server`, runs as system user
  `oct-analytics` under `oct-analytics.service` (systemd hardening enabled).
- Database: `/var/lib/oct-analytics/analytics.sqlite` (0600, dir 0700).
- TLS proxy: Caddy 2.11.7 at `/usr/local/bin/caddy`, config
  `/etc/caddy/Caddyfile`, state (certs) in `/var/lib/caddy`, unit
  `caddy.service`. Certificates renew automatically (Let's Encrypt).
- Network: DNS record `oct` = A `35.252.226.40`, **DNS only (grey cloud)** —
  keep it grey so Caddy's ACME renewals reach the origin directly; Proxied
  (orange) breaks Let's Encrypt validation. The IP is reserved as static
  address `oct-analytics-ip`; GCP firewall rule `oct-analytics-web` allows
  tcp:80,443 from 0.0.0.0/0.
- GitHub Actions variable `OCT_ANALYTICS_ENDPOINT` =
  `https://oct.suhohan.kr/v1/events`; `.goreleaser.yaml` stamps it into
  release binaries (see `.github/workflows/release.yml` validation).
- History: first deployed via a Cloudflare Tunnel (id `555603dc…`, since
  deleted along with its `cloudflared` service) and switched to the direct
  DNS + Caddy path on 2026-10-14; `~hansuho36/.cloudflared/cert.pem` on the
  host is kept for any future tunnel use.

## Common operations

```bash
# Incoming feedback (id, received_at, message; 90-day retention)
ssh gcp-free-ops "sudo sqlite3 /var/lib/oct-analytics/analytics.sqlite \
  'SELECT received_at, message FROM feedback ORDER BY id DESC;'"

# Dashboard (tailnet only)
ssh -N -L 8844:127.0.0.1:8844 gcp-free-ops   # then open http://127.0.0.1:8844/oct-analytics

# Service health / logs
ssh gcp-free-ops "systemctl status oct-analytics caddy"
ssh gcp-free-ops "journalctl -u oct-analytics -u caddy -n 50"
```

Upgrade the collector: rebuild locally
(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/oct-analytics-server`),
`scp` to the host, `install -m 0755 … /usr/local/bin/oct-analytics-server`,
`sudo systemctl restart oct-analytics`. The schema is created idempotently on
startup; no migration step needed so far.

## Related docs

- [menubar_helper_operations.md](menubar_helper_operations.md): how the app
  derives `/v1/feedback` from the stamped endpoint and how to smoke-test
  feedback locally.
- Legacy deployment (decommissioned 2026-10-14): `azure-vps` previously ran
  the collector behind a Tailscale Funnel
  (`azure-vps.tail87ad65.ts.net:8443`, user systemd unit, removed). Its data
  (430 events, 4 installations) was merged into the current database with
  `INSERT OR IGNORE` and archived at
  `azure-vps:~/oct-analytics-archive-20261014.sqlite`; delete the archive
  once the merged rows are trusted. Releases older than the one that ships
  this repo variable still post to the dead funnel URL and will silently
  drop telemetry (undelivered events are absent by design) until users
  update.
