# rh — Robinhood CLI

A single-binary, pure-HTTP Robinhood client for terminal use, scripts, and
automation.

No browser. No Selenium. No MCP server. Log in once, then everything is
plain HTTPS calls to Robinhood's REST endpoints with auto-refreshing OAuth
tokens stored locally in `~/.robinhood-cli/credentials.json`.

> **HTTP-only by design.** No browser, no Selenium, no Robinhood app
> automation. Market data, account queries, and guarded real equity order
> workflows all go through Robinhood's HTTPS APIs.

## Features

- **All accounts**: Individual, Roth IRA, Traditional IRA, Robinhood
  Strategies (Managed) — totals, cash, holdings
- **Equity market data**: real-time quotes, fundamentals, OHLCV bars
- **Options**: expirations, full chain with greeks/IV/bid-ask, current
  positions, single-contract price history
- **Crypto** (Robinhood Crypto): holdings + real-time quotes
- **Order history**: equity + option + crypto, with single-order
  per-execution drill-down
- **Real equity trading**: preview / submit / cancel guarded equity orders
- **Cash flows**: dividends received, ACH transfers in/out
- **Symbol research**: search by name, news, earnings, analyst ratings,
  similar stocks, sector tags, splits
- **Market state**: open/closed status, S&P 500 movers, watchlists
- **Account meta**: tax documents (1099), Gold subscription, margin
  balances, PDT status, notifications inbox
- **Three output formats**: `table` (human-friendly default), `plain`
  (YAML-like, AI-friendly), `json`

## Install

### Option 1: pre-built binary (recommended)

Grab the right archive for your machine from the
[Releases page](https://github.com/JackZhao98/robinhood-cli/releases) — we
publish for `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`,
`windows/arm64`, `windows/amd64` on every tag.

```bash
# example: macOS Apple Silicon
TAG=v1.0.1
curl -L "https://github.com/JackZhao98/robinhood-cli/releases/download/${TAG}/rh_${TAG#v}_darwin_arm64.tar.gz" \
  | tar -xz -C ~/.local/bin rh
chmod +x ~/.local/bin/rh
rh --version
```

Replace `darwin_arm64` with the matching slug for your platform:
`darwin_amd64`, `linux_arm64`, `linux_amd64`, `windows_arm64.zip`,
`windows_amd64.zip`.

### Option 2: install with Go (Go ≥ 1.25)

```bash
go install github.com/jackzhao/robinhood-cli/cmd/rh@latest
```

### Option 3: build from source (Go ≥ 1.25)

```bash
git clone https://github.com/JackZhao98/robinhood-cli ~/Developer/Robinhood/robinhood-cli
cd ~/Developer/Robinhood/robinhood-cli
go build -o ~/.local/bin/rh ./cmd/rh
```

Make sure `~/.local/bin` is on your `PATH`.

## First login

```bash
rh login
```

You'll be prompted for username/password. Robinhood will then trigger
one of two challenges:

- **Push notification to your phone** (Sheriff verification) — `rh` polls
  automatically every 2 s for up to 3 min, with a heartbeat to stderr
  every 15 s. Just tap "Approve" on your phone; no Enter needed.
- **6-digit MFA code** from SMS or your authenticator app — `rh` waits
  for you to type it.

Tokens are stored at `~/.robinhood-cli/credentials.json` (chmod 600).
The `device_token` is persisted at `~/.robinhood-cli/device_token` so
Robinhood treats this machine as a known device on subsequent logins
(usually no Sheriff push from then on).

`rh logout` clears stored credentials. Username/password can also be
fed via env vars:

```bash
ROBINHOOD_USERNAME=you@example.com ROBINHOOD_PASSWORD=secret rh login
```

## Quick start

```bash
# how much do I have, across all accounts?
rh account list

# what's in my Roth IRA?
rh account list                   # find the IRA's account_number
rh account show <ACCOUNT_NUMBER>

# real-time quote
rh quote NVDA

# recent buy/sell activity
rh activity --limit 20

# how much did I really make? (combine three calls — see Recipes below)
rh account snapshot
rh transfers
rh dividends
```

## Output formats

```
--format table   default — ASCII table for human terminal use
--format plain   YAML-like indented key/value, AI-friendly
--format json    pretty-printed JSON for scripts / jq pipelines
```

```bash
rh quote NVDA
rh quote NVDA --format plain
rh activity --limit 5 --format json | jq '.orders[] | .symbol'
```

## Command reference

```
AUTH
  rh login / logout

ACCOUNT
  rh account list                              # overview, no holdings
  rh account show <ACCOUNT>                    # one account, sorted by equity
  rh account snapshot                          # totals + every account's holdings

EQUITY MARKET DATA
  rh quote <SYMBOL>
  rh bars  <SYMBOL> --from YYYY-MM-DD --to YYYY-MM-DD [--interval day]

OPTIONS
  rh option expirations <SYMBOL>
  rh option chain <SYMBOL> --exp YYYY-MM-DD [--type call|put] [--side buy|sell]
  rh option positions
  rh option history <INSTRUMENT_ID> [--span week] [--interval hour]

CRYPTO
  rh crypto holdings
  rh crypto quote <SYMBOL>          # BTC, ETH, BTCUSD, ...

ORDERS / ACTIVITY
  rh activity [--limit N] [--since YYYY-MM-DD] [--asset equity|option|crypto]
  rh order <ID>                     # one order with executions[]
  rh trade buy|sell|cancel ...      # preview / submit / cancel real equity orders

CASH FLOWS
  rh dividends [--since YYYY-MM-DD] [--limit N]
  rh transfers [--since YYYY-MM-DD] [--limit N]

SYMBOL RESEARCH
  rh symbol search "company or ticker"
  rh symbol news     <SYMBOL>
  rh symbol earnings <SYMBOL>
  rh symbol ratings  <SYMBOL>
  rh symbol similar  <SYMBOL>
  rh symbol tags     <SYMBOL>
  rh symbol splits   <SYMBOL>

MARKET STATE
  rh market                         # open/closed + today's hours
  rh movers --direction up|down     # S&P 500 top gainers/losers
  rh index ...                      # market indexes (VIX, etc.)
  rh scan ...                       # Bonfire screener
  rh watchlist list / show NAME

ACCOUNT META
  rh documents [--tax-year 2025]    # 1099s and statements
  rh gold                           # Gold subscription state
  rh margin                         # margin balances per account
  rh pdt                            # day-trade count + PDT status
  rh notifications [--limit N]
META
  rh version

```

`rh --help` lists everything. `rh <command> --help` shows flags.

## Recipes

**True realized P&L** (what you actually made vs. money you put in):

```bash
PORTFOLIO=$(rh account snapshot --format json | jq '.total_portfolio')
CRYPTO=$(rh crypto holdings --format json | jq '.total_market_value')
DEPOSITED=$(rh transfers --format json | jq '.net_deposited')
DIVIDENDS=$(rh dividends --format json | jq '.total_paid')
echo "scale=2; $PORTFOLIO + $CRYPTO - $DEPOSITED + $DIVIDENDS" | bc
```

**Find the IRA's account number, then list its top 5 holdings:**

```bash
IRA=$(rh account list --format json | jq -r '.accounts[] | select(.brokerage_account_type=="ira_roth") | .account_number')
rh account show "$IRA" --format json | jq '.holdings[:5]'
```

**Filter activity to one symbol:**

```bash
rh activity --limit 200 --format json | jq '.orders[] | select(.symbol=="NVDA")'
```

## How it works

1. **Login** posts to `https://api.robinhood.com/oauth2/token/` with
   `grant_type=password`. If Robinhood demands Sheriff verification, the
   CLI starts a `pathfinder/user_machine` workflow, polls
   `/push/{challenge_id}/get_prompts_status/` until you approve on your
   phone, then finalizes and re-requests the token.
2. **Stored credentials** include `access_token`, `refresh_token`,
   `expires_at`, and a stable `device_token`. The token gets refreshed
   automatically when it's within 60s of expiry.
3. **Every other command** is a normal `GET` to api.robinhood.com or
   nummus.robinhood.com (crypto) with `Authorization: Bearer <access>`.

## Limitations / known issues

- Real trading is limited to guarded **equity** orders via `rh trade`.
- Options / crypto remain read-only.
- **Old dividend records** sometimes return without a resolvable symbol
  (Robinhood's instrument lookup may not return delisted/ancient
  instruments).
- A few endpoints (`notifications`, `gold`) are best-effort — Robinhood
  doesn't publicly document them and field names may shift. Commands
  previously included for per-account equity history and recurring DCA
  configs were dropped after Robinhood decommissioned their endpoints.
- Crypto only covers Robinhood Crypto holdings; if you've moved coins out
  to self-custody those are obviously invisible.
- Pure Go, zero CGO — `GOOS=linux GOARCH=arm64 go build` and other
  cross-compiles work without any extra toolchain.

## Releasing (maintainer notes)

Tagging a `v*` commit triggers `.github/workflows/release.yml`, which runs
GoReleaser to build all six platform binaries, archive them
(`tar.gz` for unix, `zip` for windows), generate `checksums.txt`, and
publish a GitHub Release with auto-generated changelog.

```bash
# bump and ship
git tag v1.0.2
git push origin v1.0.2
# wait ~3 min, check https://github.com/JackZhao98/robinhood-cli/releases
```

Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes
(`feat:`, `fix:`, `docs:`, `chore:`, `ci:`) so the auto-generated
changelog groups things sensibly. `docs:` / `chore:` / `ci:` commits are
filtered out of release notes by `.goreleaser.yaml`.

To dry-run the release process locally without publishing:

```bash
goreleaser release --snapshot --clean
ls dist/
```

## License

MIT.
