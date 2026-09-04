# Examples

Sanitized sample configurations and README section output for [read-waka-stats](https://github.com/vihuvac/read-waka-stats).

These files are **illustrative only**. They use fictional GitHub profile facts and WakaTime fixture-style project names (`project-alpha`, etc.). Do not commit tokens, private repository details, or markdown generated from a live account into this repository.

The sample chart at [`assets/bar_graph.png`](assets/bar_graph.png) is sanitized synthetic LOC data (not from a real profile). Root `/assets/` from local Docker debug runs stays gitignored.

Pin the Action to a release tag (for example `@v1.0.0`). Bump the tag when you upgrade.

| Example | Highlights |
| --- | --- |
| [full](full/) | Most sections enabled (default-style profile block) |
| [minimal](minimal/) | Languages and timezone only |
| [ai-and-chart](ai-and-chart/) | AI coding stats, commit hours, and LOC timeline chart |

Each directory contains:

- `config.yml` — copy-paste workflow step (`uses` + `with`)
- `output.md` — sample markdown between `<!--START_SECTION:waka-->` markers

## Local Docker note

`docker compose` / `make docker-run` defaults to **mock WakaTime** fixtures and **live GitHub** data from `INPUT_GH_TOKEN`. Treat that output as personal; do not paste it into examples or PRs.
