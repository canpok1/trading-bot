#!/bin/bash
cd $(dirname $0)/../

export $(cat configs/db.env | grep -v "^#" | xargs)
export $(cat configs/slack.env | grep -v "^#" | xargs)
export $(cat configs/exchange.env | grep -v "^#" | xargs)
export $(cat configs/bot2.env | grep -v "^#" | xargs)

go run cmd/trading-bot2/main.go $1
