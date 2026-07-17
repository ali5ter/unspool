# unspool

A subscription-first YouTube TUI — Shorts-free, distraction-free, and locally owned.

`unspool` treats your YouTube subscriptions and playlists as the primary interface: no
algorithmic home feed, no Shorts, no autoplay. It owns your feed state, queue, and watch
history locally as plain JSON, and gives AI-generated "slop" a best-effort, honestly-labelled
filter. Built in the same mould as [`wwlog`](https://github.com/ali5ter/wwlog).

The full product spec lives in [`PRD.md`](PRD.md).

## Status

**Pre-release, M1 in progress** (read-only feed). Not yet on Homebrew or GitHub Releases —
build from source for now. See [`PRD.md` §11](PRD.md#11-suggested-milestones-all-api-only) for
the milestone roadmap.

## Features (target, per milestone)

- **Shorts-free by construction** — sourced from each channel's `UULF` uploads playlist, not
  post-filtered
- **Local-first store** — plain JSON, `wwlog`-style; `--export json` is close to a straight copy
- **Best-effort AI-slop filtering** — channel mute (reliable) + metadata heuristics (advisory) +
  provenance badges (precise, low recall) + an on-demand LLM inspect hook — never asserts
  certainty
- **A locally-owned Queue** (Watch Later replacement) that auto-mirrors to a real YouTube
  playlist, and a local watch log (history replacement)
- **Synthesised recommendations** from your own subscriptions and watch history
- **mpv playback** with SponsorBlock and audio-only mode
- **Pipeline mode** — `--json`, `--export`, `--sync`, `--offline` for scripting

## Installation

```bash
go install github.com/ali5ter/unspool@latest
```

Or build from source:

```bash
git clone git@github.com:ali5ter/unspool.git
cd unspool
go build -o unspool .
```

**Runtime dependencies:** [`mpv`](https://mpv.io) (which uses `yt-dlp` as its stream backend).

```bash
# macOS
brew install mpv yt-dlp
```

## Quick start

**1. Set up a Google Cloud OAuth client** (one-time, free — see
[`docs/SETUP.md`](docs/SETUP.md) for the full walkthrough):

```bash
./scripts/setup-gcp.sh
```

**2. Authenticate** — opens your browser, stores a refresh token in your system keychain:

```bash
unspool --login
```

**3. Browse:**

```bash
# Open the TUI
unspool

# Refresh the local cache and exit (cron-friendly)
unspool --sync

# Feed as JSON, no TUI
unspool --json | jq '.[] | select(.duration_seconds > 1200)'
```

## Configuration

`~/.config/unspool/config.toml` — see [`PRD.md` §8](PRD.md#8-config-configunspoolconfigtoml)
for the full schema (playback, thumbnails, theme, Queue mirroring, AI-filter thresholds, the
model-agnostic classifier shell-out hooks).

## Why not just use the YouTube app?

Shorts can't be permanently disabled, the home feed optimises for engagement over your actual
subscriptions, and there's no filter for the growing volume of AI-generated content. See
[`PRD.md` §1](PRD.md#1-why-this-exists) for the full rationale, and
[§2](PRD.md#2-feasibility-summary-read-this-first) for what the YouTube Data API can and can't
do — and why.

## License

MIT — see [LICENSE](LICENSE).
