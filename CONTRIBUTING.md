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
11. Update [`CHANGELOG.md`](./CHANGELOG.md) for every user-facing or behavior change (see below).

## Changelog

Keep [`CHANGELOG.md`](./CHANGELOG.md) in sync with the work you ship. Follow
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

- **Same unreleased / untagged work:** append under the current heading (usually
  `[Unreleased]` or the latest dated version that has not been published yet).
  Do not invent a new version for follow-up commits on the same release.
- **New release:** when cutting a distinct version, move `[Unreleased]` entries
  into a new `## [x.y.z] - YYYY-MM-DD` section, leave an empty `## [Unreleased]`
  above it, and tag the release to match.
- **Entry shape:** optional `Related tickets:` list, then `### Added` /
  `### Changed` / `### Fixed` / `### Removed` as needed. Prefer a **bold lead-in**
  and a short explanation of *why* / impact, not only a file list.
- Skip the changelog only for pure chore noise (typos in comments, CI-only
  tweaks with no consumer impact) unless the PR also changes docs or behavior.
