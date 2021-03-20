#!/bin/bash
cd $(dirname $0)/../

export BOT_RATE_LOG_INTERVAL_SECONDS=10
export BOT_TARGET_CURRENCY=mona
export BOT_POSITION_COUNT_MAX=1

# 取引所
export BOT_EXCHANGE_ACCESS_KEY=xxxx
export BOT_EXCHANGE_SECRET_KEY=xxxx

# DB設定
export BOT_DB_HOST=db
export BOT_DB_PORT=3306
export BOT_DB_NAME=trading-bot
export BOT_DB_USER_NAME=bot
export BOT_DB_PASSWORD=P@ssw0rd

go run cmd/trading-bot/main.go $1
