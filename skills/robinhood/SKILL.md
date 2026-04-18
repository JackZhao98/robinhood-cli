---
name: robinhood
description: |
  Query Robinhood account data via the local `rh` CLI: balances, holdings,
  stock + crypto quotes, historical bars, options chains/greeks, options
  positions, order history, dividends, ACH transfers, earnings calendar,
  news, analyst ratings, watchlists, market status, documents (1099),
  Gold/margin/PDT status. Auth is one-time `rh login`
  (pure HTTP, no browser); tokens auto-refresh. Every command prints to
  stdout (default plain/YAML; --format json | table).
allowed-tools:
  - Bash
---

# Robinhood Skill

Thin wrapper around the `rh` CLI. Default output is YAML-like plain text
(best for parsing in your context). Add `--format json` only if you need
JSON (e.g. piping to `jq`); add `--format table` only when summarizing for
a human terminal user. Non-zero exit on failure with `error: ...` on stderr
(or `{"error": "..."}` if `--format json`).

## Auth

Credentials live at `~/.robinhood-cli/credentials.json` (chmod 600). On
`not logged in` / `unauthorized`, ask the user to run `rh login` themselves
in a terminal — it prompts for password and may need a phone push approval.
Do NOT run `rh login` from inside the agent session. `rh logout` clears
credentials.

## Command map

```
ACCOUNT / PORTFOLIO
  rh account list                           # all accounts overview, no holdings (cheap)
  rh account show <ACCOUNT>                 # holdings for one account, sorted by equity
  rh account snapshot                       # totals + every account's holdings (heavier)

EQUITY MARKET DATA
  rh quote <SYMBOL>                         # real-time price + fundamentals
  rh bars  <SYMBOL> --from <D> --to <D> [--interval day]

OPTIONS
  rh option expirations <SYMBOL>            # list available expiration dates
  rh option chain <SYMBOL> --exp <D> [--type call|put] [--side buy|sell]
  rh option positions                       # current option holdings
  rh option history <INSTRUMENT_ID> [--span week] [--interval hour]

CRYPTO
  rh crypto holdings                        # crypto positions + market value + return
  rh crypto quote <SYMBOL>                  # BTC, ETH, BTCUSD, etc.

ORDERS / ACTIVITY
  rh activity [--limit N] [--since YYYY-MM-DD] [--asset equity|option|crypto]
  rh order <ID>                             # one order with per-execution detail

CASH FLOWS
  rh dividends [--since YYYY-MM-DD] [--limit N]
  rh transfers [--since YYYY-MM-DD] [--limit N]

SYMBOL RESEARCH
  rh symbol search "company or ticker"      # find a ticker by name
  rh symbol news     <SYMBOL>
  rh symbol earnings <SYMBOL>               # past + upcoming reports
  rh symbol ratings  <SYMBOL>               # analyst buy/hold/sell breakdown
  rh symbol similar  <SYMBOL>
  rh symbol tags     <SYMBOL>               # sector/theme collections
  rh symbol splits   <SYMBOL>

MARKET STATE
  rh market                                 # open/closed + today's hours
  rh movers --direction up|down             # S&P 500 top gainers/losers
  rh watchlist list / rh watchlist show NAME

ACCOUNT META
  rh documents [--tax-year 2025]            # 1099/statements with download URLs
  rh gold                                   # Robinhood Gold subscription state
  rh margin                                 # margin balances per account
  rh pdt                                    # pattern day-trader count + status
  rh notifications [--limit N]
```

## Conventions

- Dates: `YYYY-MM-DD`. Symbols UPPERCASE in user-facing output.
- Money: USD floats; round to 2 decimals when displaying.
- Never dump raw output unless asked; summarize the relevant fields.

## Recipes

- **"How much do I have?"** → `rh account list` (lead with `total_portfolio`,
  one line per account).
- **"What's in my Roth IRA?"** → `rh account list` to find the IRA's
  account_number, then `rh account show <that_number>`.
- **"How is my portfolio doing this year?"** → `rh account snapshot`
  (current state) combined with the P&L recipe below.
- **"How much did I really make?"** → combine `rh account snapshot`
  (current value), `rh transfers` (sum of `net_deposited`),
  `rh dividends` (`total_paid`). True P&L = current_value - net_deposited
  + total_paid.
- **"Did I make money on NVDA?"** → look at `holdings[].total_return` from
  `rh account show <acct>` (Robinhood already does FIFO). For lot detail,
  use `rh activity --asset equity` and group by symbol.
- **"What's BTC at?"** → `rh crypto quote BTC`.
- **"Find Apple's ticker"** → `rh symbol search "apple"`.
- **"When does NVDA report?"** → `rh symbol earnings NVDA`.
- **"Is the market open?"** → `rh market` (look at `is_open_now`).
- **"What's hot today?"** → `rh movers --direction up`.
- **"My IRA contributions for tax year 2025"** → `rh transfers
  --since 2025-01-01` filtered to that account.
