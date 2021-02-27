run:
	go run cmd/trading-bot/main.go -f ./configs/bot-follow-uptrend.toml

run-sample:
	go run cmd/trading-bot/main.go -f ./configs/bot-watch-only.toml

simulation:
	go run cmd/simulator/main.go -f ./configs/simulator.toml
