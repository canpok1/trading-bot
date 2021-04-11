#!/bin/bash
cd $(dirname $0)/../

export BOT_TARGET_CURRENCY=mona
export BOT_INTERVAL_SECONDS=60
export BOT_SUPPORT_LINE_PERIOD1=120
export BOT_SUPPORT_LINE_PERIOD2=120
export BOT_AVERAGING_DOWN_RATE_PER=0.99
export BOT_MAX_VOLUME=1000.0
export BOT_VOLUME_CHECK_SECONDS=120

export BOT_FUNDS_RATIO=1.0
export BOT_FUNDS_RATIO_PER_ORDER=0.2
export BOT_TARGET_PROFIT_PER=0.005

# 取引所
export BOT_EXCHANGE_ACCESS_KEY=xxxxx
export BOT_EXCHANGE_SECRET_KEY=xxxxx

# DB設定
export BOT_DB_HOST=db
export BOT_DB_PORT=3306
export BOT_DB_NAME=trading-bot
export BOT_DB_USER_NAME=bot
export BOT_DB_PASSWORD=P@ssw0rd

go run cmd/trading-bot2/main.go $1
