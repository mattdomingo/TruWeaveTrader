# Alpaca TUI - Blazing-Fast Terminal Trading App

A high-performance terminal trading application for the Alpaca API, written in Go for ultra-low latency.

## Features

- ⚡ **Lightning-fast quotes** - Sub-150ms quote fetching with intelligent caching
- 📊 **Real-time streaming** - WebSocket streaming for live market data
- 🎯 **One-command trading** - Quick buy/sell commands with comprehensive risk checks
- 🤖 **Automated strategies** - Mean reversion, momentum, and pairs trading algorithms
- 🛡️ **Risk management** - Built-in position sizing, spread checks, and loss limits
- 📈 **Beautiful output** - Color-coded tables and formatted displays
- 🔒 **Safe by default** - Paper trading mode with explicit live confirmation

## Performance Targets

- First quote fetch: < 150ms (cold) / < 25ms (cached)
- Trade submission: < 100ms to Alpaca REST
- Streaming latency: Real-time via WebSocket

## Installation

### Prerequisites

1. Install Go 1.21 or later: https://golang.org/dl/
2. Get your Alpaca API keys from: https://alpaca.markets/

### Build from source

```bash
# Clone the repository
git clone https://github.com/TruWeaveTrader/alpaca-tui.git
cd alpaca-tui

# Install dependencies
go mod download

# Build the binary
go build -o alpaca-tui

# Or install globally
go install
```

## Configuration

Create a `.env` file in your working directory (or copy `env.example`):

```bash
# Alpaca API Configuration
ALPACA_KEY_ID=your_api_key_here
ALPACA_SECRET_KEY=your_secret_key_here
ALPACA_BASE_URL=https://paper-api.alpaca.markets
ALPACA_DATA_URL=https://data.alpaca.markets

# Trading Configuration
LIVE_TRADING=false
DEFAULT_TIMEFRAME=1Min

# Risk Management
RISK_MAX_POSITION_USD=10000.0
RISK_MAX_DAILY_LOSS_USD=1000.0
RISK_MAX_SPREAD_PERCENT=0.5

# Performance
CACHE_TTL_MS=100
WEBSOCKET_RECONNECT_DELAY_MS=1000
HTTP_TIMEOUT_MS=2000
```

## Usage

### Quick Quote
```bash
# Get a quote for AAPL
alpaca-tui q AAPL

# Or use the full command
alpaca-tui quote AAPL
```

### Account Information
```bash
# Show account summary
alpaca-tui acct

# Or
alpaca-tui account
```

### Positions
```bash
# Show all open positions
alpaca-tui pos

# Or
alpaca-tui positions
```

### Orders
```bash
# Show open orders
alpaca-tui orders

# Show all orders
alpaca-tui orders --all
```

### Trading

#### Buy
```bash
# Market order
alpaca-tui buy AAPL 100

# Limit order
alpaca-tui buy AAPL 100 --type limit --limit 150.50

# Stop order
alpaca-tui buy AAPL 100 --type stop --stop 155.00

# Extended hours
alpaca-tui buy AAPL 100 --extended
```

#### Sell
```bash
# Market sell
alpaca-tui sell AAPL 50

# Limit sell
alpaca-tui sell AAPL 50 --type limit --limit 160.00

# With time in force
alpaca-tui sell AAPL 50 --type limit --limit 160.00 --tif gtc
```

#### Cancel Order
```bash
# Cancel by order ID
alpaca-tui cancel 12345678-1234-1234-1234-123456789012
```

### Real-time Streaming
```bash
# Stream single symbol
alpaca-tui stream AAPL

# Stream multiple symbols
alpaca-tui stream AAPL MSFT TSLA GOOGL
```

### Automated Trading
```bash
# Enable automation
echo "STRATEGIES_ENABLED=true" >> .env

# Start all strategies
alpaca-tui auto start

# Check automation status
alpaca-tui auto status

# View strategy performance
alpaca-tui auto metrics

# Stop automation
alpaca-tui auto stop
```

## Trading Modes

### Paper Trading (Default)
The app starts in paper trading mode by default. All trades are simulated.

### Live Trading
To enable live trading:

1. Set `LIVE_TRADING=true` in your `.env` file
2. Restart the app
3. You'll need to type `confirm-live` when executing trades

⚠️ **WARNING**: Live trading uses real money. Use at your own risk.

## Risk Management

Built-in safety features:

- **Position size limits** - Max position size enforced
- **Daily loss limits** - Prevents excessive daily losses
- **Spread checks** - Warns on wide bid-ask spreads
- **Buying power validation** - Ensures sufficient funds
- **Pattern day trader checks** - Monitors day trading limits

## Command Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `quote [symbol]` | `q` | Get quick market snapshot |
| `account` | `acct` | Show account information |
| `positions` | `pos` | Display open positions |
| `orders` | - | Show orders |
| `buy [symbol] [qty]` | - | Buy shares |
| `sell [symbol] [qty]` | - | Sell shares |
| `cancel [order-id]` | - | Cancel an order |
| `stream [symbols...]` | - | Stream live data |
| `auto start` | - | Start automated trading |
| `auto stop` | - | Stop automated trading |
| `auto status` | - | Show automation status |
| `auto metrics` | - | Show strategy performance |

### Order Type Flags

- `--type [market|limit|stop|stop_limit]` - Order type
- `--limit [price]` - Limit price
- `--stop [price]` - Stop price
- `--tif [day|gtc|ioc|fok]` - Time in force
- `--extended` - Allow extended hours trading

## Troubleshooting

### Connection Issues
- Verify your API keys are correct
- Check if you're using the right base URL (paper vs live)
- Ensure your system time is synchronized

### Performance Issues
- Reduce the number of streaming symbols
- Increase cache TTL for less volatile instruments
- Check your network latency to Alpaca servers

## Development

### Project Structure
```
alpaca-tui/
├── cmd/            # CLI commands
├── internal/       # Internal packages
│   ├── alpaca/     # Alpaca API client
│   ├── cache/      # In-memory cache
│   ├── config/     # Configuration
│   ├── models/     # Data models
│   ├── risk/       # Risk management
│   └── websocket/  # Streaming client
├── pkg/            # Public packages
│   └── formatters/ # Output formatting
└── main.go         # Entry point
```

### Running Tests
```bash
go test ./...
```

### Building for Production
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o alpaca-tui-linux

# macOS
GOOS=darwin GOARCH=amd64 go build -o alpaca-tui-macos

# Windows
GOOS=windows GOARCH=amd64 go build -o alpaca-tui.exe
```

## Automated Trading Strategies

The system includes three built-in algorithmic trading strategies:

1. **Mean Reversion** - Trades FIG, NEGG, NVDA, META when prices deviate from moving averages
2. **Momentum** - Follows trends with stop losses on AAPL, MSFT, GOOGL, TSLA  
3. **Pairs Trading** - Statistical arbitrage on NVDA/AMD and META/GOOGL pairs

See [AUTOMATION.md](AUTOMATION.md) for detailed documentation.

## Future Enhancements

- [ ] Options trading support
- [ ] TUI interface with live panels
- [ ] Chart integration
- [x] Strategy automation (✅ **Completed**)
- [ ] Redis cache support
- [ ] Multi-account support
- [ ] Backtesting framework
- [ ] Machine learning models

## License

MIT License - see LICENSE file for details.

## Disclaimer

This software is for educational purposes only. Trading stocks involves risk, and past performance does not guarantee future results. Always do your own research and consider your financial situation before trading.
