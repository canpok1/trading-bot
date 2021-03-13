run:
	scripts/run_bot.sh scalping

run-sample:
	scripts/run_bot.sh watch_only

test:
	go test ./...

simulation:
	scripts/run_simulator.sh

ga-simulation:
	scripts/run_ga_simulator.sh
