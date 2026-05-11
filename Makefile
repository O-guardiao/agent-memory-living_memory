.PHONY: test run fmt lint zip bench

test:
	go test ./...

run:
	go run ./cmd/memory-api

fmt:
	gofmt -w ./cmd ./internal ./sdk/go

lint:
	go vet ./...

bench:
	go test -bench=. ./...

zip:
	cd .. && zip -r agent-memory.zip agent-memory -x 'agent-memory/.git/*'
