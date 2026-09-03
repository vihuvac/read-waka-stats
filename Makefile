.PHONY: test vet build

test:
	go test ./... -count=1

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -o bin/read-waka-stats ./cmd/read-waka-stats
