.PHONY: test vet build docker-run

test:
	go test ./... -count=1

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -o bin/read-waka-stats ./cmd/read-waka-stats

docker-run:
	docker compose run --build --rm app
