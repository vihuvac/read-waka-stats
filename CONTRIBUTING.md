# Contribution Guide

🎉 Thanks for taking the time to contribute! 🎉

If you'd like to improve Read Waka Stats, please feel free to fork the repository and
submit a Pull Request.

1. Fork the repo.
2. Create a new branch (`git checkout -b feature/amazing-feature`).
3. Commit your changes (`git commit -m 'feat: add amazing feature'`).
4. Push to the branch (`git push origin feature/amazing-feature`).
5. Run `go test ./...` and `go vet ./...` before opening a pull request.
6. Open a Pull Request.
7. Keep business logic in `internal/` packages with unit tests next to the code.
8. Do not add Python or other runtimes to the Docker image.
9. README list ranking must sort the full dataset before applying `SHOW_LANGUAGE_COUNT` (or other limits).
