test:
	while true; do inotifywait -e modify,close_write,moved_to,move,create,delete $$(fd .go); go test ./...; done
.PHONY: test
bench:
	go test -bench . -cpuprofile cpu.pp
	go tool pprof cpu.pp
.PHONY: bench
