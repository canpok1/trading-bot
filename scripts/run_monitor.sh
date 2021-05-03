#!/bin/bash
cd $(dirname $0)/../

export $(cat configs/db.env | grep -v "^#" | xargs)

go run cmd/monitor/main.go
