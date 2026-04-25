---
name: robinhood
description: Robinhood brokerage data and guarded equity-trading skill via the `rh` CLI.
allowed-tools:
  - Bash
---

# Robinhood Skill

Use this skill when you need Robinhood account data, market data, research,
cash-flow history, or guarded real equity trading through the `rh` CLI.

Prefer this skill for Robinhood-specific source-of-truth queries.
Do not use it as the primary handler for portfolio-wide diagnosis, strategy
planning, paper trading, or sync orchestration; let the `cfo` skill lead those
flows and call `rh` only when needed.

## Setup

Before running any `rh` command, make sure the binary is on PATH. Install it
yourself if it isn't — the binary persists across runs once it lands in
`~/.local/bin`.

```bash
if ! command -v rh >/dev/null 2>&1; then
  TAG=v1.0.1
  PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')          # linux | darwin
  ARCH=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
  mkdir -p "$HOME/.local/bin"
  curl -fsSL "https://github.com/JackZhao98/robinhood-cli/releases/download/${TAG}/rh_${TAG#v}_${PLATFORM}_${ARCH}.tar.gz" \
    | tar -xz -C "$HOME/.local/bin" rh
  chmod +x "$HOME/.local/bin/rh"
fi

rh version
```

If you have Go available and prefer building from source:

```bash
go install github.com/jackzhao/robinhood-cli/cmd/rh@latest
```

Skip the install entirely once `command -v rh` succeeds.

## Authentication

Credentials live in `~/.robinhood-cli/credentials.json` and persist across
runs once written, so login is a one-time setup per host.

`rh login` reads `ROBINHOOD_USERNAME` and `ROBINHOOD_PASSWORD` from the
environment when those flags aren't passed. **Always use the env vars** —
do not prompt the user for credentials in chat, do not pass them on the
command line (they'd land in shell history).

Before the first command in any run, ensure you're authenticated:

```bash
# Probe — succeeds quickly when a valid credentials file exists.
if ! rh account list --format plain >/dev/null 2>&1; then
  if [ -z "$ROBINHOOD_USERNAME" ] || [ -z "$ROBINHOOD_PASSWORD" ]; then
    echo "ERROR: Robinhood credentials are not configured." >&2
    echo "Set ROBINHOOD_USERNAME and ROBINHOOD_PASSWORD as environment" >&2
    echo "variables (per-app or per-user). Once set, this skill will" >&2
    echo "log in automatically on first use and persist the session." >&2
    exit 1
  fi
  rh login
fi
```

What this does:
- If there's a valid credential file, the probe succeeds and we skip
  login entirely — no extra round-trip per run.
- If there's no credential file and the env vars are set, `rh login`
  uses them and writes the credential file. Subsequent runs hit the
  fast path above.
- If there's no credential file **and** the env vars are missing, fail
  loudly with an actionable message — do not try to "make do" with
  partial auth, and do not ask the user to type a password into chat.

Notes:
- Robinhood may issue an MFA challenge during the first `rh login`. In a
  chat session this is fine — the prompt surfaces to the user and they
  reply with the code. In a scheduled / webhook run there's no human, so
  the first authentication must be primed from a chat session before
  switching the app to unattended mode.
- `rh logout` clears the local credential file. Don't run it implicitly.

## Output format

Default to:

```bash
--format plain
```

Use `--format json` only when you truly need programmatic aggregation,
filtering, matching, or numeric computation.

## Abilities

### Accounts and holdings

```bash
rh account list --format plain
rh account show ACCOUNT_NUMBER --format plain
rh account snapshot --format plain
```

Notes:
- `account list` is the lightweight overview.
- `account snapshot` is the heavy full-state pull.
- Managed accounts may appear alongside self-directed and IRA accounts.

### Quotes and price history

```bash
rh quote SYMBOL --format plain
rh quote SYMBOL1 SYMBOL2 SYMBOL3 --format plain
rh bars SYMBOL --from YYYY-MM-DD --to YYYY-MM-DD --interval day --format plain
```

### Options data

```bash
rh option expirations SYMBOL --format plain
rh option chain SYMBOL --exp YYYY-MM-DD --type put --format plain
rh option positions --format plain
rh option history INSTRUMENT_ID --format plain
```

### Activity and orders

```bash
rh activity --limit 50 --format plain
rh activity --since YYYY-MM-DD --asset equity --format plain
rh activity --account ACCOUNT_NUMBER --format plain
rh order ORDER_ID --format plain
```

### Cash flows

```bash
rh dividends --format plain
rh transfers --format plain
```

### Symbol research

```bash
rh symbol search "query" --format plain
rh symbol news SYMBOL --format plain
rh symbol earnings SYMBOL --format plain
rh symbol ratings SYMBOL --format plain
rh symbol similar SYMBOL --format plain
rh symbol tags SYMBOL --format plain
rh symbol splits SYMBOL --format plain
```

### Market and screening

```bash
rh market --format plain
rh movers --direction up --format plain
rh watchlist list --format plain
rh watchlist show NAME --format plain
rh index --list --format plain
rh index VIX --format plain
rh scan --min-change 10 --format plain
```

### Account metadata

```bash
rh documents --format plain
rh gold --format plain
rh margin --format plain
rh pdt --format plain
rh notifications --limit 20 --format plain
```

### Trading

```bash
rh trade buy SYMBOL AMOUNT
rh trade sell SYMBOL AMOUNT
rh trade cancel ORDER_ID
```

Trading rules:
- Default to preview only.
- Add `--execute` only after explicit user confirmation.
- `rh trade` currently supports real equity orders only.
- Options and crypto are data-only here.

## Working style

- Use the smallest command that answers the question.
- Prefer live `rh` data over memory or web finance for Robinhood questions.
- Do not dump raw output unless the user asks for it.
- Summarize the important fields after running commands.
- When matching trades, wash-sale windows, or multi-account activity, use
  `--format json` and compute carefully.
