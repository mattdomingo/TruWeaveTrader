# Automated Trading System

This document describes the automated trading capabilities of TruWeaveTrader, including mean reversion, momentum, and pairs trading strategies.

## Overview

The automation system provides three built-in trading strategies:

1. **Mean Reversion** - Trades when prices deviate significantly from moving averages
2. **Momentum** - Follows trends with built-in stop losses and take profits  
3. **Pairs Trading** - Exploits price divergences between correlated securities

## Quick Start

### 1. Enable Automation

```bash
# Set in your .env file
STRATEGIES_ENABLED=true
```

### 2. Start Trading

```bash
# Start all configured strategies
alpaca-tui auto start

# Check status
alpaca-tui auto status

# View metrics
alpaca-tui auto metrics
```

### 3. Monitor Performance

```bash
# List all strategies and their performance
alpaca-tui auto list

# Get detailed metrics for a specific strategy
alpaca-tui auto metrics mean_reversion_tech
```

## Built-in Strategies

### Mean Reversion Strategy

**Symbols**: FIG, NEGG, NVDA, META  
**Logic**: Buys when price drops >2.5% below 20-period moving average, sells when price returns to average  
**Risk**: Max $5,000 per position, 30-minute cooldown between trades

```bash
# Enable only mean reversion
alpaca-tui auto enable mean_reversion_tech
```

### Momentum Strategy  

**Symbols**: AAPL, MSFT, GOOGL, TSLA  
**Logic**: Follows trends using 10/30 moving average crossovers with RSI confirmation  
**Risk**: 2% stop loss, 4% take profit, max $8,000 per position

```bash
# Enable momentum strategy
alpaca-tui auto enable momentum_trending
```

### Pairs Trading Strategy

**Pairs**: NVDA/AMD, META/GOOGL  
**Logic**: Trades spread divergences using statistical mean reversion  
**Risk**: Entry at 2σ, exit at 0.5σ, stop loss at 3σ

```bash
# Enable pairs trading
alpaca-tui auto enable pairs_tech_correlation
```

## Command Reference

### Core Commands

```bash
# Start/Stop
alpaca-tui auto start [strategy-name]    # Start all or specific strategy
alpaca-tui auto stop [strategy-name]     # Stop all or specific strategy

# Status & Monitoring  
alpaca-tui auto status                   # Show automation status
alpaca-tui auto list                     # List all strategies
alpaca-tui auto metrics [strategy-name]  # Show performance metrics

# Strategy Management
alpaca-tui auto enable [strategy-name]   # Enable specific strategy
alpaca-tui auto disable [strategy-name]  # Disable specific strategy
```

### Advanced Commands

```bash
# Add custom strategy
alpaca-tui auto add my-momentum \
  --type momentum \
  --symbols AAPL,MSFT,NVDA \
  --config '{"stop_loss_percent": 1.5, "take_profit_percent": 3.0}'

# Remove strategy
alpaca-tui auto remove my-momentum
```

## Strategy Configuration

Each strategy supports customizable parameters:

### Mean Reversion Parameters

```json
{
  "lookback_period": 20,        // Moving average period
  "threshold_percent": 2.5,     // Deviation threshold for entry
  "max_position_usd": 5000.0,   // Maximum position size
  "cooldown_minutes": 30        // Time between trades
}
```

### Momentum Parameters

```json
{
  "short_period": 10,           // Short MA period
  "long_period": 30,            // Long MA period  
  "rsi_period": 14,             // RSI calculation period
  "stop_loss_percent": 2.0,     // Stop loss threshold
  "take_profit_percent": 4.0,   // Take profit threshold
  "min_momentum": 1.0,          // Minimum momentum for entry
  "max_position_usd": 8000.0,   // Maximum position size
  "cooldown_minutes": 15        // Time between trades
}
```

### Pairs Trading Parameters

```json
{
  "lookback_period": 60,        // Statistical lookback period
  "entry_threshold": 2.0,       // Z-score for entry (2σ)
  "exit_threshold": 0.5,        // Z-score for exit (0.5σ)
  "stop_loss_threshold": 3.0,   // Z-score for stop loss (3σ)
  "max_position_usd": 10000.0,  // Maximum position size per pair
  "cooldown_minutes": 30,       // Time between trades
  "pairs": [
    {"symbol1": "NVDA", "symbol2": "AMD", "ratio": 1.0},
    {"symbol1": "META", "symbol2": "GOOGL", "ratio": 1.0}
  ]
}
```

## Risk Management

All strategies integrate with the existing risk management system:

- **Position size limits** - Enforced per strategy and globally
- **Daily loss limits** - Automatically stops trading if exceeded  
- **Spread checks** - Validates bid-ask spreads before execution
- **Buying power validation** - Ensures sufficient funds
- **Paper trading mode** - Test strategies safely

## Scheduling

Strategies run during market hours (9:30 AM - 4:00 PM ET, Monday-Friday) by default. This can be customized in the strategy configuration.

## Performance Metrics

The system tracks comprehensive metrics for each strategy:

- **Total trades** - Number of completed trades
- **Win rate** - Percentage of profitable trades  
- **Total P&L** - Cumulative profit/loss
- **Average win/loss** - Average profit per winning/losing trade
- **Max drawdown** - Largest peak-to-trough loss
- **Sharpe ratio** - Risk-adjusted returns (when available)

## Safety Features

### Emergency Stop

```bash
# Immediately stop all automated trading
alpaca-tui auto stop
```

### Live Trading Protection

When using live trading mode, you'll be prompted to confirm with `confirm-live` before any trades execute.

### Position Monitoring

All positions are continuously monitored for:
- Stop loss triggers
- Take profit targets  
- Risk limit breaches
- Market hours compliance

## Troubleshooting

### Common Issues

1. **"Strategies disabled"** - Set `STRATEGIES_ENABLED=true` in your `.env` file
2. **"No historical data"** - Ensure your Alpaca API keys have market data access
3. **"Risk check failed"** - Check position sizes and daily loss limits
4. **"Spread too wide"** - Strategy skipped trade due to poor liquidity

### Logs

Strategy execution is logged with structured logging. Check logs for detailed information about signal generation and trade execution.

### Support

For issues or questions about the automation system, check:
1. This documentation
2. Application logs  
3. Alpaca API status
4. Your account permissions and buying power

## Disclaimer

Automated trading involves significant risk. This software is for educational purposes only. Always test strategies in paper trading mode before using real money. Past performance does not guarantee future results.

## Example Workflow

Here's a complete example of setting up and running automated trading:

```bash
# 1. Configure environment
echo "STRATEGIES_ENABLED=true" >> .env

# 2. Start the automation system
alpaca-tui auto start

# 3. Monitor status
alpaca-tui auto status

# 4. Check performance after some time
alpaca-tui auto metrics

# 5. Stop when done
alpaca-tui auto stop
```

The system will automatically load the pre-configured strategies and begin monitoring the markets for trading opportunities within your risk parameters. 