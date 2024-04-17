test:
	while true; do inotifywait -e modify,close_write,moved_to,move,create,delete $$(fd .go); go test ./...; done
.PHONY: test
