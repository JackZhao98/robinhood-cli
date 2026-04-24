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

## Install

`rh` can be installed in either of these ways:

```bash
go install github.com/jackzhao/robinhood-cli/cmd/rh@latest
```

```bash
TAG=v1.0.1
curl -L "https://github.com/JackZhao98/robinhood-cli/releases/download/${TAG}/rh_${TAG#v}_linux_arm64.tar.gz" \
  | tar -xz -C ~/.local/bin rh
chmod +x ~/.local/bin/rh
```

After install:

```bash
rh version
```

## Authentication

- Credentials live in `~/.robinhood-cli/credentials.json`.
- If the user is not logged in, tell them to run `rh login` themselves.
- Do not run `rh login` inside the agent session.
- `rh logout` clears local credentials.

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
