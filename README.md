# rh — Robinhood CLI

A single-binary, pure-HTTP, read-only Robinhood client. Built to be
**driven by AI assistants** (via a Claude Code Skill) but works just as
well as a regular terminal tool.

No browser. No Selenium. No MCP server. Log in once, then everything is
plain HTTPS calls to Robinhood's REST endpoints with auto-refreshing OAuth
tokens stored locally in `~/.robinhood-cli/credentials.json`.

> **Read-only by design.** This tool will never place, modify, or cancel a
> trade. It only fetches data.

## Features

- **All accounts**: Individual, Roth IRA, Traditional IRA, Robinhood
  Strategies (Managed) — totals, cash, holdings, equity history
- **Equity market data**: real-time quotes, fundamentals, OHLCV bars
- **Options**: expirations, full chain with greeks/IV/bid-ask, current
  positions, single-contract price history
- **Crypto** (Robinhood Crypto): holdings + real-time quotes
- **Order history**: equity + option + crypto, with single-order
  per-execution drill-down
- **Cash flows**: dividends received, ACH transfers in/out, recurring
  investments (DCA)
- **Symbol research**: search by name, news, earnings, analyst ratings,
  similar stocks, sector tags, splits
- **Market state**: open/closed status, S&P 500 movers, watchlists
- **Account meta**: tax documents (1099), Gold subscription, margin
  balances, PDT status, notifications inbox
- **Three output formats**: `plain` (YAML-like, AI-friendly default),
  `json`, `table`

## Install

### Option 1: pre-built binary (recommended)

Grab the right archive for your machine from the
[Releases page](https://github.com/JackZhao98/robinhood-cli/releases) — we
publish for `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`,
`windows/arm64`, `windows/amd64` on every tag.

```bash
# example: macOS Apple Silicon
TAG=v0.1.0
curl -L "https://github.com/JackZhao98/robinhood-cli/releases/download/${TAG}/rh_${TAG#v}_darwin_arm64.tar.gz" \
  | tar -xz -C ~/.local/bin rh
chmod +x ~/.local/bin/rh
rh --version
```

### Option 2: build from source (Go ≥ 1.25)

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

You'll be prompted for username/password. Robinhood may then either:

- Send a **push notification to your phone** (Sheriff verification) —
  approve it, then press Enter in the terminal.
- Ask for a **6-digit MFA code** from SMS or your authenticator app.

Tokens are stored at `~/.robinhood-cli/credentials.json` (chmod 600).
The `device_token` is persisted at `~/.robinhood-cli/device_token` so
Robinhood treats this machine as a known device on subsequent logins.

`rh logout` clears stored credentials.

You can also pass credentials non-interactively via env vars:

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

# how is my main account doing this year?
rh account history --account <ACCT> --span year

# recent buy/sell activity
rh activity --limit 20

# how much did I really make? (combine three calls — see Recipes below)
rh account snapshot
rh transfers
rh dividends
```

## Output formats

```
--format plain   default — YAML-like indented key/value, AI-friendly
--format json    pretty-printed JSON for scripts / jq pipelines
--format table   ASCII table; for nested data, falls back to plain
```

```bash
rh quote NVDA --format table
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
  rh account history --account X --span year [--interval day]

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

CASH FLOWS
  rh dividends [--since YYYY-MM-DD] [--limit N]
  rh transfers [--since YYYY-MM-DD] [--limit N]
  rh recurring                      # DCA configurations

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
  rh watchlist list / show NAME

ACCOUNT META
  rh documents [--tax-year 2025]    # 1099s and statements
  rh gold                           # Gold subscription state
  rh margin                         # margin balances per account
  rh pdt                            # day-trade count + PDT status
  rh notifications [--limit N]
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

## Use as an AI Skill (Claude Code)

The repo ships with a Skill so Claude can drive `rh` automatically. See
`~/.claude/skills/robinhood/SKILL.md` (or copy it from the project's
example).

Once installed, ask Claude things like:

- "How much do I have across all my Robinhood accounts?"
- "What's in my Roth IRA?"
- "How is my portfolio doing this year? How does that compare to my
  net deposits?"
- "What's BTC at right now?"
- "Find the ticker for SoFi and pull the latest news."

Claude will call `rh` under the hood, parse the JSON, and summarize.

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

- **Read-only**. By design — no order placement.
- **Old dividend records** sometimes return without a resolvable symbol
  (Robinhood's instrument lookup may not return delisted/ancient
  instruments).
- A few endpoints (`notifications`, `gold`, `recurring`) are best-effort
  — Robinhood doesn't publicly document them and field names may shift.
- Crypto only covers Robinhood Crypto holdings; if you've moved coins out
  to self-custody those are obviously invisible.
- macOS-only build instructions above; the Go source has no platform
  dependencies, so `GOOS=linux go build` etc. work fine.

## Releasing (maintainer notes)

Tagging a `v*` commit triggers `.github/workflows/release.yml`, which runs
GoReleaser to build all six platform binaries, archive them
(`tar.gz` for unix, `zip` for windows), generate `checksums.txt`, and
publish a GitHub Release with auto-generated changelog.

```bash
# bump and ship
git tag v0.1.0
git push origin v0.1.0
# wait ~2 min, check https://github.com/JackZhao98/robinhood-cli/releases
```

To dry-run the release process locally without publishing:

```bash
goreleaser release --snapshot --clean
ls dist/
```

## License

MIT.
