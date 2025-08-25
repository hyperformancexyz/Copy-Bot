# Copy-Bot
<img width="2048" height="889" alt="image" src="https://github.com/user-attachments/assets/0d67c270-a264-4d33-85a0-26f89b03bfd1" />

**Copy-Bot** is a copy trading bot for [Hyperliquid](https://hyperliquid.xyz).  
It mirrors trades from a *copy account* into a *paste account* in real time and includes a terminal UI for monitoring.

---

## Features

- **Copy-Trading Engines**
  - **IOC Engine** – mirrors partially filled or cancelled IOC orders.
  - **ALO Engine** – reconciles Add-Liquidity-Only orders between copy and paste.
- **Resilient WebSocket Client** – handles reconnects and streams `webData2`, `orderUpdates`, and `l2Book`.
- **Terminal UI (TUI)** – built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss):
  - Split log panes for copy and paste accounts.
  - Orders view with CLOID tracking and side coloring.
  - Positions view with PnL, RoE, margin %, entry and mid prices.

---

## Installation

Requires **Go 1.23+** and a Hyperliquid account.

```bash
git clone https://github.com/hyperformancexyz/Copy-Bot
cd Copy-Bot
go run main.go
```

## Configuration

Create a `config.json` (or copy from `config.sample.json`):

```
{
  "secret_key": "YourPrivateKeyWithout0x",
  "account_address": "0xYourPasteAddress",
  "copy_address": "0xAddressToFollow",
  "coins": { "BTC": 1.0 },
  "disable_alo_engine": true,
  "disable_ioc_engine": false
}
```
## Usage

Run the bot:

`go run main.go -c config.json`


The TUI shows:
	•	Logs – split view (left = copy, right = paste).
	•	Orders Pane – active orders with CLOIDs and side coloring.
	•	Positions Pane – leverage, margin %, PnL, RoE, entry and mid prices.
	•	Status Bar – balance and unrealized PnL with flash indicators.

Press q or Ctrl+C to quit.


⸻

## Development

Key components:
	•	main.go – entrypoint, sets up manager, engines, and TUI.
	•	ws/ – WebSocket client, IOC engine, ALO engine, reconciliation logic.
	•	tui/ – terminal UI for logs, orders, positions.
	•	utils/ – logger and formatting helpers.
	•	config.sample.json – example configuration.

⸻

## Notes
	•	Dependencies: this project depends on the fork itay747/go-hyperliquid.
	•	Security: never commit your private key or a real config.
	•	Reproducibility: pin commits in go.mod for deterministic builds.

## License

MIT
