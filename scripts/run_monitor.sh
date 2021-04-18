#!/bin/bash
cd $(dirname $0)/../

# DB設定
export MONITOR_DB_HOST=db
export MONITOR_DB_PORT=3306
export MONITOR_DB_NAME=trading-bot
export MONITOR_DB_USER_NAME=bot
export MONITOR_DB_PASSWORD=P@ssw0rd

go run cmd/monitor/main.go
