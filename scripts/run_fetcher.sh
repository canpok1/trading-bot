#!/bin/bash
cd $(dirname $0)/../

export $(cat configs/db.env | grep -v "^#" | xargs)
export $(cat configs/fetcher.env | grep -v "^#" | xargs)

go run cmd/market-fetcher/main.go
