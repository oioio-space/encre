# Deploy — ENCRE production (ticket T31, brief/ENCRE_04 §12)

One Hetzner CX22 (4 €/mois), one systemd unit for the game server, one for
Litestream, Caddy in front. No Docker: see `deploy/encre.service`'s header
comment for why a Dockerfile lost to systemd + a static binary for this
topology.

## Build

```
mise run wasm:build     # → dist/web/{index.html,sw.js,static/*}
mise run server:build   # → dist/bin/encre-server
mise run native:build   # → dist/bin/encre-client-{linux,darwin,windows}-*  (optional desktop client)
```

## Install (first time)

```
useradd --system --home /var/lib/encre --create-home encre
install -o encre -g encre -m 0755 dist/bin/encre-server /usr/local/bin/encre-server
mkdir -p /var/lib/encre/web
rsync -a --delete dist/web/ /var/lib/encre/web/current/
chown -R encre:encre /var/lib/encre

mkdir -p /etc/encre
# Pepper key (encre-qpx.4) — generate once, OUTSIDE /var/lib/encre so it is
# never inside Litestream's replicated path (deploy/litestream.yml's dbs.path):
python3 -c "import secrets, base64; print('v1:' + base64.b64encode(secrets.token_bytes(32)).decode())"
# → paste as ENCRE_PEPPER_KEY= below
cat > /etc/encre/pepper.env <<'EOF'
ENCRE_PEPPER_KEY=v1:<paste the base64 key here>
EOF
chmod 0600 /etc/encre/pepper.env

install -m 0644 deploy/encre.service /etc/systemd/system/encre.service
install -m 0644 deploy/Caddyfile /etc/caddy/Caddyfile   # edit the hostname first
systemctl daemon-reload
systemctl enable --now encre
systemctl reload caddy
```

Then Litestream — see `deploy/litestream.yml`'s header comment for the exact
package to install (`v0.3.14`, not latest) and why, and
`deploy/encre-litestream.env.example` for the B2 and age-key variables to
fill in. **Run `deploy/restore-test.sh` against the real B2 bucket before
calling this done** — it already passed against a local file replica
standing in for B2 during this ticket's work (same Litestream code path);
that is not the same as this VPS's real credentials working.

## Trusted proxies, end to end

`server/auth.ClientIP(r, trustedProxies)` only reads `X-Forwarded-For` when
the immediate TCP peer is in `trustedProxies`; otherwise every request keys
the rate limiter on `RemoteAddr` (see `server/auth/clientip.go`). In this
topology Caddy is the only thing between the internet and `cmd/server`, and
Caddy always connects to it from loopback — so `cmd/server`'s
`trustedProxies` must be exactly `{127.0.0.1/32, ::1/128}` wherever
`server/api` ends up wiring `ClientIP` into a handler (not yet done as of
this ticket — `server/api.Server.Handler()` has no rate-limited route yet;
this is the value to pass in when it does).

Caddy's own `trusted_proxies` global option is a **different** concern —
whether Caddy trusts an *incoming* `X-Forwarded-For` from whatever connects
to *it* — and is deliberately left unset in `deploy/Caddyfile`, because
nothing sits in front of Caddy here. Verified against the current Caddy docs
(`https://caddyserver.com/docs/caddyfile/options#trusted-proxies`, checked
2026-09-14) rather than assumed. If a CDN is ever added in front of Caddy,
both sides of this mirror move: Caddy's `trusted_proxies` gets the CDN's
published ranges, and `cmd/server`'s `trustedProxies` still stays
`{127.0.0.1/32, ::1/128}` — Caddy remains the only thing `cmd/server` itself
ever talks to directly.

What does not depend on any of this: Caddy's `reverse_proxy` always
*appends* the real immediate peer's address as the last hop of
`X-Forwarded-For` before forwarding — it does not relay a client-supplied
value unmodified — so `ClientIP`'s right-to-left walk finds the real client
even if an attacker prepends a forged entry, independent of whether Caddy's
own `trusted_proxies` is set.

## Litestream: why v0.3.14, and what it protects

`encre-qpx.4` requires the B2 replication target encrypted end to end: a
leaked B2 application key must not be enough to read `children.pattern_hash`,
`parents.pass_hash` or `parents.totp_secret` (already peppered/encrypted at
rest by `server/auth.Pepper` — see its doc comment — but a second,
independent layer on the transport is what this ticket owns). Litestream's
current release line (`v0.5.x`) **removed** age encryption support during its
LTX storage rewrite; its own binary refuses the config with a pointer back to
`v0.3.x`. `v0.3.14` (the newest `0.3` tag) still has it, verified by hand
during this ticket:

```
$ deploy/restore-test.sh "$(go env GOPATH)/bin/litestream" # built @v0.3.14
== checking the replica is actually encrypted, not just compressed ==
OK: snapshot begins with the age-encryption.org/v1 header
== restoring WITHOUT the identity: must fail ==
OK: restore without the identity failed, as required
== restoring WITH the identity: must succeed and match the original ==
RESTORE DRILL: OK
```

Litestream itself is installed from the official `.deb`
(`litestream-v0.3.14-linux-amd64.deb`), not `go install`: v0.3.14 needs CGO
(`mattn/go-sqlite3`), and building it inside this repo's own
`CGO_ENABLED=0` environment fails outright — which is correct, that rule is
scoped to `github.com/oioio-space/encre`'s own module, and Litestream is
explicitly a separate process operating on the WAL at the file level
(litestream.io's own documented usage), never an embedded dependency.

### Losing the age key

The age identity (secret key) must exist in at least two places that cannot
both fail at once: a password manager entry, and — if you want a second
copy on paper or a second device — anywhere that is not this VPS. Losing it
does not lose data immediately (the primary `/var/lib/encre/encre.db` is
still there), but it makes every past and future B2 backup permanently
unreadable, forever, by design — that is what "leaking the B2 key alone
must not be enough" costs.

## `/admin/metrics`

`GET /admin/metrics`, gated by `server/auth.SessionFromCookie(...,
auth.CookieParent, ...)` — the same parent-session cookie every other parent
route uses, no separate access control invented (`cmd/server/admin.go`).
Computes four of ENCRE_04 §12's six numbers directly in SQL
(`cmd/server/metrics.go`): rank distribution, failures by rank and manche,
average time per word, and the blind-attempt share. The response's
`limitations` field documents the other two (gold words per week, and
revanche counts) and exactly why they are not derivable from the schema as
it stands — see `cmd/server/metrics.go`'s package doc comment.

## The WASM budget: measured, and revised

ENCRE_04 §2 budgets ~3 MB brotli. Measured on this ticket's build
(`mise run wasm:build`, `-trimpath -ldflags="-s -w"`, `go1.27.1`):

| | raw | gzip -9 | brotli -q11 |
|---|---:|---:|---:|
| `cmd/client` wasm | ~25 MB | ~7.2 MB | ~4.1 MB |

`-s -w` (stripped debug info) saves only ~3%: most of the weight is not
debug symbols, it is `go-text/typesetting`'s shaping engine and Unicode
tables, pulled in the moment `GoTextFace` loads a real TTF — confirmed
against an earlier build in this same session (23 MB raw / 3.7 MB brotli vs.
this one's 25 MB / 4.1 MB; the delta between those two runs is unrelated
client-side work landing concurrently during this ticket, not this
ticket's own change — the shape of the number, not its exact value, is the
point).

Levers checked and rejected:

- **TinyGo** (`GOOS=js`, `-target wasm`): would shrink the binary
  substantially, but `tinygo version` on this machine is 0.41.1, which
  requires Go 1.19–1.26 — this project pins Go 1.27.1
  (`mise.toml [tools] go`), and TinyGo refuses outright:
  `requires go version 1.19 through 1.26, got go1.27`. Tested, not assumed.
  Worth re-checking whenever TinyGo catches up to 1.27, but not viable today
  without downgrading the whole project's Go version — a much larger call
  than this ticket's scope.
- **`wasm-opt` (binaryen)**: not installed on this machine, not evaluated;
  it trims dead code and can help on top of the Go compiler's own output,
  but does not touch the Unicode/shaping tables that dominate the size, so
  it would be a smaller win than either of the above, not a different order
  of magnitude.
- **Elision of Unicode tables in `go-text/typesetting`**: would need a
  patched fork of a third-party dependency to drop scripts ENCRE never
  renders (there is no upstream build tag for this) — real leverage, but a
  change to someone else's library, not a build flag; flagging it for
  `architect`/`go-dev` rather than doing it inside a deploy ticket.

**Recommendation: revise the ENCRE_04 §2 budget from ~3 MB to ~4 MB brotli.**
The 3 MB figure predates real TTF rendering; `go-text/typesetting` is the
already-decided architecture (ENCRE_07 §5 rejected the alternatives for real
reasons), and the levers that exist without forking a dependency or
downgrading the Go toolchain buy single-digit percent, not the ~25% this
would need. The service worker (`web/sw.js`) already caches the binary after
first load, so the ~4 MB cost is paid once per browser, not once per run —
which is what makes it a number to budget rather than a number to chase
further right now.
