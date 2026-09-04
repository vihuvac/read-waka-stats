# Contribution Guide

🎉 Thanks for taking the time to contribute! 🎉

If you'd like to improve Read Waka Stats, please feel free to fork the repository and
submit a Pull Request.

1. Fork the repo.
2. Create a new branch (`git checkout -b feature/amazing-feature`).
3. Commit your changes (`git commit -m 'feat: add amazing feature'`).
4. Push to the branch (`git push origin feature/amazing-feature`).
5. Run `go test ./...` and `go vet ./...` before opening a pull request (`make test` / `make vet`).
6. Optionally exercise a full local debug run with Docker: copy `.env.example` to `.env`, set `INPUT_GH_TOKEN`, then `make docker-run` (uses `Dockerfile.dev` + Compose; skips publish via `DEBUG_RUN`).
7. Open a Pull Request. To re-run CI tests on a feature branch without a new push, use **Actions → CI → Run workflow**.
8. Keep business logic in `internal/` packages with unit tests next to the code.
9. Do not add Python or other runtimes to the production Docker image (`Dockerfile`).
10. README list ranking must sort the full dataset before applying `SHOW_LANGUAGE_COUNT` (or other limits).
