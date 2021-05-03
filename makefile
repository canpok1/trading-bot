run:
	scripts/run_bot.sh inago

run2:
	scripts/run_bot2.sh

run-logger:
	scripts/run_bot.sh none

run-monitor:
	scripts/run_monitor.sh

run-fetcher:
	scripts/run_fetcher.sh

test:
	go test ./...

simulation:
	scripts/run_simulator.sh

ga-simulation:
	scripts/run_ga_simulator.sh
