# Changelog

All notable changes to this project will be documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.2] - 2026-11-05

Related tickets:

- No tickets were registered for this update.

### Changed

- **WakaTime failures:** Weekly and all-time fetch errors abort the action (exit non-zero) instead of soft warnings that left the README unchanged, so GitHub can notify on failure.
- **Docs:** README troubleshooting covers personal OAuth tokens (plugin-only API keys as of November 2026) and the short automatic retry when stats are still calculating.

### Fixed

- **WakaTime stats still calculating:** Poll the stats endpoints when the API returns HTTP 201/202 or `is_up_to_date: false`, up to five attempts with a 5s pause (~20s total). If data is still not ready, fail with a clear “still calculating after N attempts” error; other HTTP/network/decode failures fail immediately with status and response detail.
