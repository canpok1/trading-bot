#!/bin/bash
cd $(dirname $0)/../

export BOT_TARGET_CURRENCY=mona
export BOT_INTERVAL_SECONDS=60
export BOT_DEMO_MODE=true

export BOT_TREND_LINE_PERIOD=150
export BOT_TREND_LINE_OFFSET=5
export BOT_ENTRY_AREA_WIDTH=0.005

export BOT_BREAKOUT_RATIO=0.003
export BOT_AVERAGING_DOWN_RATE_PER=0.98
export BOT_SELL_MAX_VOLUME=2000.0
export BOT_BUY_MAX_VOLUME=2000.0
export BOT_VOLUME_CHECK_SECONDS=120
export BOT_SOARED_WARNING_PERIOD_SECONDS=0
export BOT_BUY_INTERVAL_SECONDS=600

export BOT_FUNDS_RATIO=1.0
export BOT_FUNDS_RATIO_PER_ORDER=0.2
export BOT_TARGET_PROFIT_PER=0.005

# 取引所
export BOT_EXCHANGE_ACCESS_KEY=xxxxxxxxxx
export BOT_EXCHANGE_SECRET_KEY=xxxxxxxxxx

# DB設定
export BOT_DB_HOST=db
export BOT_DB_PORT=3306
export BOT_DB_NAME=trading-bot
export BOT_DB_USER_NAME=bot
export BOT_DB_PASSWORD=P@ssw0rd

# Slack設定
export BOT_SLACK_URL=https://xxxxxxxxxx

go run cmd/trading-bot2/main.go $1
