# unspool

A TUI to browse YouTube subscriptions and playlists — Shorts-free, distraction-free, and
locally owned.

`unspool` treats your YouTube subscriptions and playlists as the primary interface: no
algorithmic home feed, no Shorts, no autoplay. It owns your feed state, queue, and watch
history locally as plain JSON, and gives AI-generated "slop" a best-effort, honestly-labelled
filter.

## Status

**Pre-release, M1 in progress** (read-only feed). Not yet on Homebrew — build from source or
grab a [release binary](https://github.com/ali5ter/unspool/releases) for now.

## Features (target, per milestone)

- **Shorts-free by construction** — sourced from each channel's `UULF` uploads playlist, not
  post-filtered
- **Local-first store** — plain JSON on disk; `--export json` is close to a straight copy
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

`~/.config/unspool/config.toml` (macOS: `~/Library/Application Support/unspool/config.toml`).
Key settings:

```toml
store_dir              = ""            # local store path (default: alongside config)
max_resolution         = 1080
audio_only_default     = false
playback_detached      = true
thumbnails             = "auto"        # "auto" | "chafa" | "halfblock" | "off"
theme                  = "warm-dark"
view_mode              = "rows"        # "rows" | "grid"
cookies_from_browser   = ""            # playback auth only; "" | "firefox" | "chrome" | "safari"
sponsorblock           = ["sponsor", "selfpromo", "interaction"]

[queue]
mirror                 = true          # keep the Queue synced to a real playlist

[filters]
hide_shorts            = true
ai_score_threshold     = 0.7           # dim/hide feed items scoring above this (0 = off)
ai_autohide            = false         # false = badge+dim; true = hide outright

[classifier]
# Model-agnostic shell-out hooks for AI-slop inspection. Empty = metadata heuristics only.
transcript_command     = ""
inspect_command        = ""
```

## Why not just use the YouTube app?

Shorts can't be permanently disabled, the home feed optimises for engagement over your actual
subscriptions, and there's no reliable filter for the growing volume of AI-generated content —
the platform only flags self-declared synthetic media, which misses most AI voiceover,
AI-written scripts, and faceless AI channels entirely. `unspool` can't fix YouTube itself, but
it can make sure none of that ever reaches its own view of your subscriptions.

## License

MIT — see [LICENSE](LICENSE).
