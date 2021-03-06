include local.env
export

run:
	go run cmd/trading-bot/main.go follow_uptrend

run-sample:
	go run cmd/trading-bot/main.go watch_only

test:
	go test ./...

simulation:
	go run cmd/simulator/main.go
