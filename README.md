# Read Waka Stats

GitHub Action that refreshes your profile README with WakaTime activity and GitHub development metrics.

Implemented in Go, packaged as a Docker action, and designed to update a marked section of your README on a schedule.

<div align="center">
  <img src="https://github.com/vihuvac/read-waka-stats/actions/workflows/ci.yml/badge.svg" alt="Tests Status" />
  <img src="https://codecov.io/gh/vihuvac/read-waka-stats/branch/main/graph/badge.svg" alt="Coverage" />
  <img src="https://img.shields.io/badge/Security%20Policy-Active-success" alt="Security Policy" />
  <img src="https://img.shields.io/badge/Docker-Ready-blue" alt="Docker Ready" />
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License" />
  <img src="https://img.shields.io/badge/Code%20of%20Conduct-Contributor%20Covenant-yellow" alt="Code of Conduct" />
</div>

## Features

- Weekly WakaTime languages, editors, projects, operating systems, and timezone
- All-time coding time and AI coding badges
- Weekly AI coding breakdown (sessions, tokens, model mix, insights)
- Commit hour profile (early bird vs night owl) and most productive weekday
- Language distribution across repositories
- Lifetime lines-of-code badge and quarterly PNG timeline chart
- GitHub profile facts and profile-view badge
- 22 UI locales
- Lists are **sorted on the full dataset, then truncated** so “top N” is always the true ranking
- **Verified commits** via GitHub’s `createCommitOnBranch` GraphQL API (works with signed-commit rulesets)

## Preview

Place markers in your profile README. The action replaces everything between them:

```markdown
<!--START_SECTION:waka-->
<!--END_SECTION:waka-->
```

Stats are written as markdown: shields.io badges, fenced `text` progress bars (reliable in GitHub README rendering), and a committed PNG for the lines-of-code timeline.

### Examples

Sanitized sample configs and section output (no live account data):

- [examples/](examples/README.md) — index
- [full](examples/full/) — most sections enabled
- [minimal](examples/minimal/) — languages and timezone only
- [ai-and-chart](examples/ai-and-chart/) — AI stats, commit hours, and LOC chart

## Quick start

### 1. Secrets

In the profile repository (`USERNAME/USERNAME`):

| Secret | Purpose |
| --- | --- |
| `WAKATIME_API_KEY` | From [WakaTime API settings](https://wakatime.com/settings/api-key) |
| `GH_TOKEN` | Optional. Omit to use `${{ github.token }}` for API reads (and publish when `PUSH_TOKEN` is also omitted). Use a PAT for private-repo stats and profile views |
| `PUSH_TOKEN` | Optional write token for verified publish only. Use when `main` requires PRs and the token owner (or GitHub Actions) is on the ruleset bypass list |

### 2. README markers

Add the section comments shown above to `README.md`.

### 3. Workflow

Create `.github/workflows/waka-stats.yml`:

```yaml
name: Update WakaTime stats

on:
  schedule:
    - cron: "0 0 * * *"
  workflow_dispatch:

permissions:
  contents: write

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: vihuvac/read-waka-stats@v1.0.0
        with:
          WAKATIME_API_KEY: ${{ secrets.WAKATIME_API_KEY }}
```

Pin a release tag such as `@v1.0.0` (prefer tags over `@main`). Bump the tag when you upgrade.

Omit `GH_TOKEN` to use `${{ github.token }}`. Grant `contents: write` so the action can publish README and chart updates. For protected default branches that require pull requests, set `PUSH_TOKEN` to a PAT whose owner can bypass that rule (or allow the GitHub Actions app to bypass).

## Verified commits

A token authenticates API access; it does **not** by itself mark a normal `git push` as Verified.

This action publishes README and chart updates with GitHub’s [`createCommitOnBranch`](https://github.blog/changelog/2021-09-13-a-simpler-api-for-authoring-commits/) mutation. GitHub GPG-signs those commits, so they appear as **Verified** and satisfy “Require signed commits” rulesets.

| Token | Commit author | Verified? |
| --- | --- | --- |
| `${{ github.token }}` (default when `PUSH_TOKEN` omitted and `GH_TOKEN` uses default) | `github-actions[bot]` | Yes |
| PAT / fine-grained token / GitHub App installation token | Token owner / app | Yes |

Commits are **append-only** on `PUSH_BRANCH_NAME` (or the repository default branch). Concurrent tip changes are retried a few times.

If the target branch requires pull requests, `createCommitOnBranch` is rejected unless the publish identity is on the ruleset bypass list. Pass `PUSH_TOKEN` for that identity, or allow GitHub Actions to bypass when using `${{ github.token }}`.

## Configuration

Booleans accept `true`, `false`, `1`, `0`, `yes`, and `no`.

| Input | Default | Description |
| --- | --- | --- |
| `GH_TOKEN` | `${{ github.token }}` | GitHub token for API reads; also used for verified publish when `PUSH_TOKEN` is omitted |
| `PUSH_TOKEN` | empty → `GH_TOKEN` | Optional token for clone and verified commit publish only |
| `GH_USER` | token owner | Username to collect stats for |
| `WAKATIME_API_KEY` | required | WakaTime API key |
| `WAKATIME_API_URL` | `https://wakatime.com/api/v1/` | API base URL |
| `SECTION_NAME` | `waka` | Marker name in `<!--START_SECTION:…-->` |
| `PUSH_BRANCH_NAME` | default branch | Branch that receives the verified commit |
| `SHOW_OS` | `true` | Operating system list |
| `SHOW_PROJECTS` | `true` | Project list |
| `SHOW_EDITORS` | `true` | Editor list |
| `SHOW_TIMEZONE` | `true` | WakaTime timezone |
| `SHOW_COMMIT` | `true` | Commit-hour distribution |
| `SHOW_LANGUAGE` | `true` | Language list |
| `SHOW_LINES_OF_CODE` | `false` | Lifetime LOC badge |
| `SHOW_LANGUAGE_PER_REPO` | `true` | Language-per-repository list |
| `SHOW_LOC_CHART` | `true` | Quarterly LOC PNG under `assets/bar_graph.png` |
| `SHOW_DAYS_OF_WEEK` | `true` | Most productive weekday |
| `SHOW_PROFILE_VIEWS` | `true` | Profile traffic badge |
| `SHOW_SHORT_INFO` | `true` | GitHub facts block |
| `SHOW_UPDATED_DATE` | `true` | Footer timestamp |
| `SHOW_TOTAL_CODE_TIME` | `true` | All-time coding badge |
| `SHOW_AI_CODE_TIME` | `true` | All-time AI coding badge |
| `SHOW_AI_CODING` | `true` | Weekly AI section |
| `COMMIT_MESSAGE` | `Updated with Dev Metrics` | Verified commit message |
| `LOCALE` | `en` | UI language |
| `UPDATED_DATE_FORMAT` | `%d/%m/%Y %H:%M:%S` | strftime-style timestamp |
| `IGNORED_REPOS` | empty | Comma-separated repo names to skip |
| `MAX_REPOS` | `0` | Cap on repositories (`0` = all) |
| `SHOW_LANGUAGE_COUNT` | `5` | Languages shown after a full descending sort |
| `SYMBOL_VERSION` | `1` | Progress-bar glyphs (`1`, `2`, or `3`) |
| `BADGE_STYLE` | `flat` | shields.io style |
| `DEBUG_LOGGING` | runner debug | Verbose logs |

### Debug / dry run

Set `DEBUG_RUN=true` in the job environment to skip clone/publish and print generated markdown (or write `README_CONTENT` when `GITHUB_OUTPUT` is set).

Set `MOCK_WAKATIME=true` to load fixtures from `MOCK_DATA_DIR` instead of calling WakaTime.

## Locales

`ar`, `bn`, `ca`, `de`, `en`, `es`, `fa`, `fr`, `gl`, `hi`, `id`, `it`, `ko`, `pl`, `pt`, `ru`, `sw`, `tr`, `uk`, `vn`, `zh`, `zh_TW`

## Development

Unit tests and a local binary:

```bash
make test
make vet
make build
```

### Local Docker (debug run)

Run the action locally with Compose so you can exercise README generation without pushing a PR. Defaults: `DEBUG_RUN=true` (print markdown, skip clone/publish) and `MOCK_WAKATIME=true` (fixtures under `internal/testdata`). A real `INPUT_GH_TOKEN` is still required for GitHub API calls.

Local output is **hybrid**: WakaTime sections come from fixtures; GitHub sections use the live API for the token owner (or `INPUT_GH_USER`). Do not commit that markdown, root `assets/` chart PNGs from debug runs, or tokens into this repository—see [examples/](examples/README.md) for sanitized samples instead.

```bash
cp .env.example .env
# Set INPUT_GH_TOKEN (and optionally INPUT_GH_USER) in .env

docker compose build
docker compose run --rm app
# or: make docker-run
```

Production Action packaging stays in `Dockerfile`. Local iteration uses `Dockerfile.dev` (Go toolchain + bind-mounted source).

Layout:

```
cmd/read-waka-stats/   entrypoint
internal/app/          orchestration
internal/config/       action inputs
internal/wakatime/     WakaTime API
internal/githubx/      GitHub REST + GraphQL (including verified commits)
internal/commits/      commit aggregation
internal/render/       markdown
internal/chart/        Pure Go PNG timeline
internal/gitops/       clone + verified publish
internal/i18n/         embedded translations
```

Requirements: Go 1.24+.

## Security

- Prefer `contents: write` on `github.token` for public profile repos.
- Fine-grained tokens: Metadata + Contents read for debug/API reads; Contents read+write for publish; Administration read when profile views are enabled.
- `PUSH_TOKEN` can be a dedicated write token so `GH_TOKEN` stays read-oriented.
- Do not log API keys. The WakaTime key is sent as a query parameter because that is how the WakaTime API authenticates.

## License

MIT. Inspired by community README stats actions, implemented independently in Go.
