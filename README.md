# unspool

A TUI to browse YouTube subscriptions and playlists — Shorts-free, distraction-free, and
locally owned.

`unspool` treats your YouTube subscriptions and playlists as the primary interface: no
algorithmic home feed, no Shorts, no autoplay. It owns your feed state, queue, and watch
history locally as plain JSON, and gives AI-generated "slop" a best-effort, honestly-labelled
filter.

![unspool demo](examples/unspool_demo.gif)

## Features

- **Shorts-free by construction** — sourced from each channel's dedicated long-form uploads
  playlist, never fetched in the first place
- **Local-first** — subscriptions, queue, watch history, and playlist state all live as plain
  JSON you own
- **Best-effort AI-slop filtering** — channel mute, an advisory heuristic score, provenance
  badges, and an on-demand LLM inspect hook (`i`) — never claims certainty
- **A real Queue and watch log** — local replacements for YouTube's long-discontinued Watch
  Later/history APIs; the Queue auto-mirrors to a real YouTube playlist
- **Synthesised Recommended tab** — built from your own subscriptions and watch history, not
  YouTube's algorithm
- **mpv playback**, with an audio-only mode
- **Thumbnails in the preview pane** — rendered via `chafa`, configurable or off
- **Pipeline mode** — `--json`, `--sync`, `--export {json,csv,markdown}`, and `--offline`

## Installation

```bash
brew install ali5ter/tap/unspool
```

Or install via Go:

```bash
go install github.com/ali5ter/unspool@latest
```

Or build from source:

```bash
git clone git@github.com:ali5ter/unspool.git
cd unspool
go build -o unspool .
```

**Runtime dependencies:** [`mpv`](https://mpv.io) (which uses `yt-dlp` as its stream backend),
and optionally [`chafa`](https://hpjansson.org/chafa/) for preview-pane thumbnails (absent —
or `thumbnails = "off"` in config — just disables thumbnails, nothing else).

```bash
# macOS
brew install mpv yt-dlp chafa
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

# Export the feed — json, csv, or markdown — to a file (default: stdout)
unspool --export csv -o feed.csv

# Read the local cache with zero API calls (combine with --json or --export)
unspool --json --offline
```

## Configuration

`~/.config/unspool/config.toml` (macOS: `~/Library/Application Support/unspool/config.toml`).
Key settings:

```toml
store_dir              = ""            # local store path (default: alongside config)
max_resolution         = 1080
audio_only_default     = false
playback_detached      = true
thumbnails             = "auto"        # "auto" | "chafa" | "halfblock" | "off" — "auto" and
                                        # "chafa" render identically (both are chafa's
                                        # symbol/truecolor art); see Features
cookies_from_browser   = ""            # playback auth only; "" | "firefox" | "chrome" | "safari"
sponsorblock           = ["sponsor", "selfpromo", "interaction"]  # wired, currently a no-op — see Features

[queue]
mirror                 = true          # keep the Queue synced to a real playlist

[recommendations]
enabled                = true          # synthesised locally from your subs + watch log, no API cost

[filters]
# Shorts are already excluded structurally, via each channel's UULF uploads
# playlist — hide_shorts is a duration-based fallback guard for the rare
# channel that has no UULF playlist and falls back to the plain UU one.
hide_shorts             = true
ai_score_threshold      = 0.7          # badge/hide feed items scoring above this (0 = off)
ai_autohide             = false        # false = badge only; true = hide outright

[classifier]
# Model-agnostic shell-out hooks for AI-slop inspection — working examples
# in scripts/, see below. Empty = metadata heuristics only.
transcript_command       = ""          # tier 1: auto-run on new channels' newest video during --sync
inspect_command          = ""          # tier 2: on-demand, press `i` on a selected video
auto_inspect_new_channels = false
cache_verdicts            = true
```

## Scripts

`scripts/` holds what Quick start step 1 runs, plus working examples for the `[classifier]`
hooks above — `unspool` just shells out to whatever command you configure, so these are
reference implementations, not hardcoded behaviour:

| Script | Purpose |
| --- | --- |
| `setup-gcp.sh` | Creates/selects a GCP project and enables the YouTube Data API v3 (Quick start step 1). |
| `inspect-gemini.sh` | Example `classifier.inspect_command` (tier 2) — asks Gemini whether the selected video looks AI-generated, on-demand via `i`. |
| `inspect-transcript-gemini.sh` | Example `classifier.transcript_command` (tier 1) — judges a new channel's newest video from its auto-generated transcript alone, run automatically during `--sync` when `auto_inspect_new_channels = true`. |

The two Gemini scripts need `curl`, `jq`, `yt-dlp`, and a
[`GEMINI_API_KEY`](https://aistudio.google.com/apikey) — export it, or copy
`scripts/.env.template` to `scripts/.env` (gitignored) and fill it in there so it survives
across terminal sessions.

## Why not just use the YouTube app?

Shorts can't be permanently disabled, the home feed optimises for engagement over your actual
subscriptions, and there's no reliable filter for the growing volume of AI-generated content —
the platform only flags self-declared synthetic media, which misses most AI voiceover,
AI-written scripts, and faceless AI channels entirely. `unspool` can't fix YouTube itself, but
it can make sure none of that ever reaches its own view of your subscriptions.

## License

MIT — see [LICENSE](LICENSE).
