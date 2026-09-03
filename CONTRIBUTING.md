# Contributing

1. Run `go test ./...` and `go vet ./...` before opening a pull request.
2. Keep business logic in `internal/` packages with unit tests next to the code.
3. Do not add Python or other runtimes to the Docker image.
4. README list ranking must sort the full dataset before applying `SHOW_LANGUAGE_COUNT` (or other limits).
