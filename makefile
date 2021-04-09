run:
	scripts/run_bot.sh inago

run-logger:
	scripts/run_bot.sh none

test:
	go test ./...

simulation:
	scripts/run_simulator.sh

ga-simulation:
	scripts/run_ga_simulator.sh
