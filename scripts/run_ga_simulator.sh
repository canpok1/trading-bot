#!/bin/bash
cd $(dirname $0)/../

export BOT_RATE_LOG_INTERVAL_SECONDS=10

# 取引所
export BOT_EXCHANGE_ACCESS_KEY=xxxx
export BOT_EXCHANGE_SECRET_KEY=xxxx

# DB設定
export BOT_DB_HOST=simulation-db
export BOT_DB_PORT=3306
export BOT_DB_NAME=trading-bot
export BOT_DB_USER_NAME=bot
export BOT_DB_PASSWORD=P@ssw0rd

go run cmd/ga-simulator/main.go
