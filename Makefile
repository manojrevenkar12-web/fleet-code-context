.PHONY: test eval bench report check all

test:
	go vet ./...
	go test ./...

eval:
	go run ./cmd/fcc eval -heldout

bench:
	go run ./cmd/fcc bench

report:
	go test -json ./... > reports/tests.jsonl
	go run ./cmd/fcc report
	cp reports/results.json reports/bench.json web/public/

check:
	go run ./cmd/fcc check

all: test eval bench report
