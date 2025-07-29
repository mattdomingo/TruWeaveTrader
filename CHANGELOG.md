# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Alpaca TUI trading application
- Real-time market data streaming via WebSocket
- Lightning-fast quote retrieval with intelligent caching
- Comprehensive trading commands (buy, sell, cancel)
- Risk management with position sizing and loss limits
- Account management and portfolio tracking
- Beautiful terminal output with color-coded displays
- Paper trading mode with explicit live trading confirmation
- Support for market, limit, stop, and stop-limit orders
- Configurable risk parameters via environment variables
- Cross-platform builds (Linux, macOS, Windows)

### Features
- ⚡ Sub-150ms quote fetching (cold) / Sub-25ms (cached)
- 📊 Real-time WebSocket streaming
- 🎯 One-command trading with risk validation
- 🛡️ Built-in safety features and spread checks
- 📈 Professional terminal interface
- 🔒 Safe paper trading by default

### Commands
- `quote/q [symbol]` - Get market snapshots
- `account/acct` - Show account information
- `positions/pos` - Display current positions
- `orders` - View and manage orders
- `buy/sell [symbol] [qty]` - Execute trades
- `cancel [order-id]` - Cancel orders
- `stream [symbols...]` - Real-time data streaming

### Technical
- Written in Go for ultra-low latency
- Alpaca Markets API integration
- In-memory caching with configurable TTL
- Comprehensive error handling
- Automated testing and CI/CD
- Cross-platform support

## [0.1.0] - 2024-07-28

### Added
- Initial development version
- Core trading functionality
- Basic CLI interface
- Paper trading implementation 