# PRD: `unspool` — a subscription-first YouTube TUI

> A slick Charmbracelet TUI to browse your subscriptions, curate playlists, and watch
> YouTube — Shorts-free, distraction-free, and locally owned. Built in the same
> mould as [`wwlog`](https://github.com/ali5ter/wwlog): local-first archive, offline
> mode, pipeline/export flags, keychain auth, Homebrew + binary distribution.

*Name: `unspool`. Module `github.com/ali5ter/unspool`, binary `unspool`, shipped as a new formula in the existing tap `ali5ter/homebrew-tap` (`brew install ali5ter/tap/unspool`).*

---

## 0. Decisions locked for this handoff

- **Data access:** Official YouTube Data API v3 **only** for v1 (OAuth + quota-free RSS).
  ToS-clean — appropriate for a public repo. The yt-dlp cookie read-path
  (subs/WL/history/recs) is a **planned v2 upgrade**, explicitly *not* in v1.
- **Store:** plain JSON, `wwlog`-style (no SQLite). Hand-rolled indices accepted (§6.2).
- **In v1:** thumbnails, the LLM inspect hook (`i`), the Queue **auto-mirrored** to a real
  user-created playlist, and **synthesised** "from your subs" recommendations.
- **Name:** `unspool` — binary `unspool`, module `github.com/ali5ter/unspool`, installed via the existing `ali5ter/homebrew-tap`. (Film/reel connotation: let a video *unspool* at your pace.)

Everywhere below, ignore the hybrid/cookie option except where flagged as the v2 path;
v1 = API-only.

---

## 1. Why this exists

YouTube's own surfaces are hostile to the way you actually use it: Shorts are
force-injected and can't be permanently disabled, AI-generated "slop" is multiplying with
no consumer-side filter (unlike DeviantArt's opt-out), and the home feed optimises for
engagement rather than the subscriptions and playlists you deliberately curate. Existing
terminal clients don't fix this cleanly (see §3), and none are built on your stack.

The core idea: **treat your subscriptions and playlists as the primary interface**, own
the state locally, kill Shorts by construction, and give AI-slop a best-effort filter that
is honest about its limits.

---

## 2. Feasibility summary (read this first)

This section is the research payoff. It states plainly what is achievable, what is not,
and the workaround for each dead-end. Nothing below should be "discovered" mid-build.

### 2.1 What works

| Capability | Mechanism | Cost |
|---|---|---|
| Subscription feed | Per-channel uploads-playlist RSS, or `playlistItems.list` | RSS: **free**. API: 1 unit/page |
| **Shorts-free feed** | Use the `UULF`-prefixed uploads playlist, not `UU` | free / 1 unit |
| Read your own playlists + items | `playlists.list(mine=true)`, `playlistItems.list` | 1 unit/page |
| Create playlist, add/remove videos | `playlists.insert`, `playlistItems.insert/delete` | 50 units each |
| Like / unlike a video | `videos.rate` | 50 units |
| Read your liked videos | `videos.list(myRating=like)` | 1 unit/page |
| Video metadata (duration, AI flag) | `videos.list(part=snippet,contentDetails,status)`, batch 50 IDs | 1 unit/call |
| Keyword search | `search.list` | **100 units** — rationed |
| Playback (stream, not download) | `mpv` → `yt-dlp` backend | free |
| Skip sponsor segments | SponsorBlock via mpv Lua script or `yt-dlp --sponsorblock-mark` | free |
| Auth / age-gated / members video | `yt-dlp --cookies-from-browser` | free |

### 2.2 What the official API can't do — but yt-dlp can

The Data API abandoned three whole categories in 2016. These are *API* limitations, not
hard product limits — yt-dlp reaches them via browser cookies (the **v2 path**, §6.0) — but
**v1 is API-only, so v1 does hit them** and fills them with local equivalents:

- **Watch Later** — Data API: `WL` returns empty on read, `playlistForbidden` on write
  since 12 Sep 2016. *(v2 via yt-dlp `:ytwatchlater`.)*
  → **v1:** the auto-mirrored "▶ Queue" playlist (§5.4) — API-writable, and better anyway.
- **Watch history** — Data API: `HL` empty since 2016, `activities.list` deprecated.
  *(v2 via yt-dlp `:ythistory`.)*
  → **v1:** the local watch log populated by mpv launches (§5.4).
- **Recommendations / home feed** — Data API: `activities.list?home=true` deprecated 2016,
  returns empty. *(v2 via yt-dlp `:ytrec`.)*
  → **v1:** **synthesised** "because you're subscribed to X" recommendations from your own
  feed + watch log (§5.8). Not YouTube's algorithm — a local approximation.

These local equivalents are more portable and fully yours anyway — consistent with the
`wwlog` data-liberation ethos.

### 2.3 What genuinely doesn't work, by any route

- **There is no reliable "is this AI-generated" signal.** The API exposes
  `status.containsSyntheticMedia`, but (a) it is **self-declared** by creators and widely
  under-reported, and (b) it only covers *realistic, potentially-deceptive* synthetic
  media (deepfakes, cloned voices of real people). YouTube **explicitly exempts** the
  things you actually mean by "slop": AI voiceover narration, AI thumbnails, AI-written
  scripts, and faceless-AI-channel content. So the flag will miss ~all of it.
  → **Workaround:** treat AI-slop filtering as a *heuristic + curation* problem, not a
  lookup. See §5.2. Be honest in the UI that it's fuzzy.

- **You cannot disable Shorts on YouTube itself.** The tool can only ensure Shorts never
  appear in *its own* views (via `UULF` or a duration filter). YouTube.com is unchanged.

### 2.4 Fragility to plan for (not blockers, but budget for them)

- The `UULF`/`UUSH`/`UULV` playlist prefixes are **undocumented** by Google. They've been
  stable for years but could vanish. Keep a duration+aspect-ratio fallback (§5.1).
- Native RSS feeds return only the latest **~15 items** and are occasionally rate-limited
  or briefly unavailable. Fine for "new since last sync"; not a backfill mechanism.
- **Cookie-based reads (yt-dlp path) are the fragile part in 2026.** Chrome's app-bound
  cookie encryption has broken `--cookies-from-browser chrome` for many users; Firefox or a
  dedicated secondary browser logged into YouTube is the reliable route. Cookie feed access
  also triggers bot-detection ("confirm you're not a bot") and carries a small account-flagging
  risk when automated against your primary account. Prefer a dedicated account/browser.
- `yt-dlp` breaks periodically when YouTube changes its player/feeds. Mitigation: pin a
  known-good version, expose `--update-ytdlp`, surface yt-dlp stderr in the status bar.
- The 10,000 unit/day quota (API path) is per Google Cloud project and cannot be bought up.
  Stay RSS-first and treat `search.list` (100 units) as scarce.

---

## 3. Prior art (and why it doesn't cover this)

| Tool | Stack | Data source | Gap for this use case |
|---|---|---|---|
| `ytfzf` | POSIX shell + fzf | scraping / no API | Unmaintained (final release Jan 2024); search-first, not subscription-first; no curation state |
| `youtube-tui` | Rust | Invidious instances | Invidious is increasingly blocked/flaky in 2026; not your stack |
| `ytsub` | Rust + SQLite | Invidious | Closest in spirit (subscriptions-only) but Invidious-dependent; no AI filtering; not your stack |
| `ytui-music` | Rust | — | Music-only |
| `magic-tape` | shell + fzf | scraping | Search/browse; no owned state |

**The gap:** nothing is Go/Charmbracelet, nothing filters AI content, nothing is built
around *your* authenticated subscriptions+playlists with a locally-owned queue and watch
log. Most avoid the official API by leaning on Invidious — which trades quota limits for
reliability problems. Given you want to *manage* your own playlists (a write operation) and
value owning your data, the official OAuth API is the correct backbone here, with RSS as
the quota-free read path.

---

## 4. Goals / non-goals

**Goals**
- Subscription-first browsing with **zero Shorts**, ever.
- Best-effort **AI-slop suppression** via channel mute + heuristics, honestly labelled.
- First-class **playlist management** (view, create, add, remove, reorder).
- A locally-owned **Queue** (Watch-Later replacement, auto-mirrored to a real playlist) and
  **watch log** (history replacement).
- **Synthesised recommendations** from your own subs + watch log (a local approximation).
- **Watch** via mpv with SponsorBlock and optional audio-only mode.
- `wwlog`-grade polish: tabs, filter/sort, keychain auth, pipeline/export, offline mode.

**Non-goals (v1)**
- Downloading/archiving video files (streaming only; revisit later).
- Comments, live chat, uploading, Studio/analytics.
- Reproducing YouTube's *actual algorithmic* recommendations (needs the v2 cookie path;
  v1 synthesises its own — see §5.8).
- The yt-dlp cookie read-path for subs/WL/history/recs — deferred to v2.
- Mobile / non-terminal UI.

---

## 5. Functional requirements

### 5.1 Subscription feed — Shorts-free by construction

- On sync, resolve the user's subscriptions once via `subscriptions.list(mine=true)`
  (paginate, 1 unit/page, 50/page) and cache each channel's `UC…` ID + title.
- For each channel, derive the **long-form** uploads playlist ID by taking the 22-char
  suffix of the `UC…` ID and prefixing `UULF`
  (`UCxxxx…` → `UULFxxxx…`). Fetch new items from
  `https://www.youtube.com/feeds/videos.xml?playlist_id=UULF{suffix}` (quota-free) for the
  incremental "new since last sync", falling back to `playlistItems.list` when a deeper
  pull is needed.
- Because `UULF` = uploads − Shorts − live, **Shorts are excluded at the source**. No
  post-filtering required for the common case.
- **Fallback guard** (in case the prefix breaks): flag any item as a probable Short if
  duration ≤ 180s *and* portrait aspect ratio, and hide it. Duration comes from
  `contentDetails.duration` (ISO-8601) via a batched `videos.list`.
- Feed view: reverse-chronological, grouped by day, columns for channel, title, duration,
  age, and state badges (● new, ▶ queued, ✓ watched, 🤖 AI-flagged, 🔇 muted-channel).
- Mark-as-seen semantics stored locally; a video is "new" until viewed in the feed.

### 5.2 AI-slop filtering — honest heuristics, not a lookup

Reminder from §2.2: there is no authoritative signal. Implement layered, user-tunable
suppression and **never claim certainty**:

1. **Channel mute list (primary, sticky).** A `m` action on any row mutes the channel;
   muted channels drop out of the feed permanently until unmuted. This is the reliable
   lever — most AI-slop is channel-consistent.
2. **Heuristic score (secondary, advisory).** Compute a 0–1 "likely AI" score from cheap
   signals already fetched: title patterns (e.g. all-caps clickbait, emoji stuffing,
   templated "In this video…"), description boilerplate, suspiciously high upload cadence,
   generic/duplicated thumbnails, brand-new channel + high output. Surface as a 🤖 badge and
   an optional "dim/hide score > threshold" filter. Threshold lives in config.
3. **`containsSyntheticMedia` badge (tertiary).** Show it when present, but document in
   `--help` that it only catches declared deepfake-style content, not narration/slop.
4. **On-demand LLM inspection (the strong lever, used sparingly).** A multimodal model
   watching the actual video is a far better slop-detector than metadata — but it's
   probabilistic and too slow/costly to run inline on the whole feed. Structure it as tiers,
   all model-agnostic via the `[classifier]` shell-out hook so no provider is baked in:
   - **Tier 1 (batchable, cheap):** pull the transcript (yt-dlp auto-subs — it's just text)
     and send to any LLM for a judgment. Runs opportunistically on new/unknown channels.
   - **Tier 2 (on-demand only):** an "inspect" action (`i`) on a selected video sends the
     URL to a multimodal model that can ingest video directly (e.g. Gemini accepts a YouTube
     URL natively) — or extracted keyframes+audio for other models — and returns a verdict,
     reasoning, and *suspected* tools. Cache the result per video ID.
   - **Present as advisory, never verdict.** Tool attribution especially is educated guessing
     unless a watermark is present; label it as such. High recall, low precision.
5. **Provenance signals (precise but low recall).** Surface `status.containsSyntheticMedia`
   and, where readable, C2PA Content Credentials / SynthID. These are trustworthy *when
   present* but only catch declared or Google-tool/C2PA-tagged content — they miss most
   third-party slop (non-Google TTS + image model = no readable watermark). Show as a
   high-confidence badge; never treat their *absence* as "not AI".

Design principle: two complementary signal types — **provenance** (precise, low recall) and
**LLM reasoning** (high recall, low precision) — plus the reliable human lever, **channel
mute**. The tool **suppresses** slop from *your* view and *assists* your judgment; it never
asserts a video "is AI" as fact. Every auto-hide is reversible and shown in a
"hidden this session" count.

### 5.3 Playlists

- List the user's playlists (`playlists.list(mine=true)`), open one to see its items.
- Add current/selected video to a playlist (`playlistItems.insert`, 50 units).
- Remove item (`playlistItems.delete`, 50 units); reorder (`playlistItems.update`).
- Create a new playlist (`playlists.insert`).
- Show item count and whether a playlist is the tool's own Queue.
- **Note:** system playlists (Watch Later `WL`, History `HL`) are excluded — they error on
  access (§2.2). Liked videos (`LL`) are read-only-ish; expose as a read view via
  `videos.list(myRating=like)`.

### 5.4 Queue (Watch-Later replacement) + watch log (history replacement)

- **Queue:** a locally-owned ordered list of videos to watch. `a` adds, `d` removes,
  reorderable. Persist locally (JSON). **Auto-mirrored** to a real user-created playlist
  (v1 decision) so it's visible in the YouTube app too: on first run, create (or adopt) a
  playlist named e.g. "▶ unspool Queue" and keep it in sync via `playlistItems.insert/delete`
  (50 units each — low frequency, fine within quota). Reconcile local↔remote on sync;
  local order is source of truth. Config can point at an existing playlist or disable mirroring.
- **Watch log:** every mpv launch records `{video_id, title, channel, started_at,
  completed?}` locally. Powers a History tab and de-emphasises already-watched items in the
  feed. This is *your* data, exportable — the API can't give you YouTube's real history.

### 5.5 Search (rationed)

- `search.list` is 100 units — the day's budget is ~100 searches total. Show remaining
  quota in the status bar and warn near exhaustion.
- Prefer cheaper paths first: search *within* the local cache (subscriptions, playlists,
  watch log) for free; only hit `search.list` for genuine web-wide discovery on explicit
  intent.
- Results are actionable: play, queue, add-to-playlist, mute-channel, like.

### 5.6 Playback

- Launch `mpv <url>`; mpv uses `yt-dlp` as its stream backend automatically.
- Config for max resolution (`ytdl-format`), audio-only mode (`--no-video`, good for
  podcast-style content and your workshop/keyboard practice sessions), and playback speed.
- **SponsorBlock:** ship guidance to install the mpv `sponsorblock.lua` script, or shell
  out to `yt-dlp --sponsorblock-mark all` for chapter marks. Config `sponsorblock =
  ["sponsor","selfpromo","interaction"]`.
- **Cookies:** `--cookies-from-browser chrome` (configurable) to defeat "sign in to confirm
  you're not a bot" and reach age-restricted/members content. Surface a clear error if mpv
  or yt-dlp is missing, with install hints.
- Playback is fire-and-forget (spawn detached) or blocking-with-status; make it a config
  choice. Record the launch in the watch log.

### 5.7 Pipeline / export mode (mirror `wwlog`)

- `--json` dumps the feed/queue/playlist as a JSON array to stdout (no TUI), jq-friendly.
- `--export {json,csv,markdown}` writes to `-o/--output`.
- `--sync` refreshes the local cache and exits (for cron).
- `--no-tty` forces pipeline mode.
- Example: `unspool --sync && unspool feed --json | jq '.[] | select(.duration > 1200)'`

### 5.8 Synthesised recommendations (v1 — a local approximation, not YouTube's algorithm)

YouTube's real recommendation feed needs the v2 cookie path (§2.2). v1 builds its own from
data it already has, and labels it clearly as such:

- **Signals (all local, free):** channels you watch most (from the watch log), channels you
  haven't watched recently (resurface), videos from subscribed channels you skipped, and
  "more from a channel you just finished". Optionally cluster channels by co-watch to power
  "because you watched X".
- A **Recommendations** tab (or a section atop Feed) surfaces these with a plain-language
  "why" line ("you watch a lot of $CHANNEL"). No black box.
- Explicitly *not* a claim to reproduce YouTube's ranking. When the v2 cookie path lands,
  `:ytrec` can supplement or replace this; keep the interface stable so it can swap in.

---

## 6. Architecture

### 6.0 Data-access strategy — LOCKED: API-only for v1

**v1 uses the official YouTube Data API v3 exclusively** (plus quota-free RSS for the feed).
ToS-clean, stable, no cookie fragility — the right posture for a public repo. The consequence
is the three API gaps in §2.2, filled by local equivalents (Queue, watch log, synthesised
recs). This was chosen over the hybrid/cookie approach deliberately.

**Deferred to v2 (documented so the design leaves room):** a yt-dlp `--cookies-from-browser`
read-path that unlocks the *real* subscription feed, Watch Later, history, and
recommendations (`:ytsubs`/`:ytwatchlater`/`:ythistory`/`:ytrec`). It's ToS-gray and fragile
(Chrome cookie breakage, bot-detection — §2.4), so it stays opt-in and out of v1. Keep the
data-source layer behind an interface so a cookie-backed reader can slot in later without
touching the TUI.

For reference, the fork that was decided:

| | Reads subs/WL/history/recs | Writes (manage playlists, like) | Setup | Stability | ToS |
|---|---|---|---|---|---|
| **A. Official API only** ✅ **v1** | ✗ (filled locally) | ✓ clean | GCP project + OAuth | high | clean |
| **B. yt-dlp + cookies only** | ✓ everything | ✗ no write path | cookies only | medium | gray |
| **C. Hybrid** | ✓ via yt-dlp | ✓ via API | cookies + OAuth | med/high | mixed → **v2** |

Composable extras worth wiring in regardless: **SponsorBlock** (segment skipping) and
**DeArrow** (crowd-sourced de-clickbaited titles/thumbnails — attacks the clickbait dimension
of the AI-slop problem via a clean API, complementing §5.2).

### 6.1 Stack

- **Language:** Go (matches `wwlog`, `carrybag-lite`).
- **TUI:** Charmbracelet — Bubble Tea (runtime), Lip Gloss (style), Bubbles
  (list/table/viewport/textinput/spinner/paginator/help/key), Glamour (render video
  descriptions/markdown), Huh (first-run config + OAuth wizard), charmbracelet/log.
- **HTTP/API:** `google.golang.org/api/youtube/v3` + `golang.org/x/oauth2/google`.
- **RSS:** `mmcdole/gofeed` (Atom) for the quota-free feed path.
- **Playback:** shell out to `mpv` (which invokes `yt-dlp`). Both are external runtime deps,
  documented in README; detect-and-guide if absent.

### 6.2 Storage — LOCKED: plain JSON, `wwlog`-style

Plain JSON on disk, consistent with `wwlog`. No SQLite. The relational-ish needs (feed
dedup, mute joins, watch-state, queue order) are handled with in-memory Go maps built from
the JSON on load — at personal scale (a few hundred subs, thousands of videos) this is
trivially fast; hand-rolled indices are the accepted tradeoff.

Suggested layout under the platform config dir (`store_dir` overridable for Dropbox/iCloud
sync, same as `wwlog`):

```
store/
  subscriptions.json     # [{channel_id, title, uploads_lf_playlist_id, muted, last_seen}]
  videos/<channel_id>.json   # cached video metadata per channel (sharded to keep files small)
  feed_state.json        # per-video: seen/new, hidden, ai_score, synthetic_flag
  queue.json             # ordered [video_id]; mirrored to the remote playlist
  watch_log.json         # append-only [{video_id, title, channel, started_at, completed}]
  mutes.json             # [channel_id]
  verdicts.json          # cached LLM inspect results keyed by video_id
  playlists_cache.json   # snapshot of user playlists
```

Write atomically (temp file + rename) to survive interrupts. `watch_log.json` is
append-mostly; everything else is load-modify-rewrite. Keep a `schema_version` field per
file for painless migrations. This *is* the export format — `--export json` is close to a
straight copy, so the data-liberation promise is automatic.

### 6.3 Auth — OAuth 2.0 installed-app, keychain-stored (required from M1)

v1 is API-only, so OAuth is needed up front for both reads (own subs, playlists, likes) and
writes (manage playlists, rate).

- Scopes: `youtube.readonly` (reads) + `youtube` (manage playlists, rate); `youtube.force-ssl`
  if needed for writes.
- **Flow:** installed-app **loopback redirect** (`http://127.0.0.1:<random-port>`). Google
  has retired the out-of-band (OOB) flow, so loopback is the supported CLI pattern.
- Store the refresh token in the **system keychain** (`99designs/keyring`), exactly like
  `wwlog --login` / `--logout`.
- **Headless/SSH caveat (you've hit this class of bug before):** the loopback callback needs
  a browser on the same host. For remote/SSH use, document two fallbacks: (a) SSH port-forward
  the loopback port, or (b) run `--login` once locally and sync the token via `store_dir`.
  Do **not** rely on OOB paste — it's dead. Make the failure mode explicit, not a hang.
- The user must supply their own Google Cloud OAuth client ID/secret (free). Ship a
  `docs/SETUP.md` walking through project creation + enabling YouTube Data API v3. This is
  unavoidable — there's no way to embed a shared client for a data-scoped app safely.

### 6.4 Quota discipline (design invariant)

RSS-first for the feed. Batch `videos.list` (50 IDs/call). Cache aggressively; only
re-fetch metadata for unseen IDs. Never poll on a timer — sync on explicit action or
`--sync` cron. Track spent units in-process and show remaining budget in the status bar.

---

## 7. TUI design — familiar as the web app, better and lovely

**Design intent:** keep YouTube's spatial mental model (nav rail · content · hover-preview) so
muscle memory transfers, then use the terminal to remove everything the user dislikes and add
the craft the web app lacks. "Better" = strip Shorts/ads/mixes/autoplay + surface state the
web hides. "Lovely" = a calm cohesive palette, hierarchy by weight/colour not size, restrained
chrome, designed states and micro-motion.

Feed view (schematic — rich-row mode):

```
┌─ unspool · feed ───────────────────────────────────────────── ⌕ search ─┐
│ ▸ feed          │ new today                          │  ┌────────────┐  │
│   recommended   │▎the genius of the CEM3340 osc…     │  │     ▸      │  │
│   queue      12 │▎rick beato · 2h · 18:24 · ● new    │  │      18:24 │  │
│   playlists     │                                    │  └────────────┘  │
│   liked         │ kosmo VCF: SSI2164 wasp filter     │  the genius of   │
│   history       │ look mum no computer · 5h ·●new ▸q │  the CEM3340…    │
│                 │                                    │  rick beato·4.9M │
│ SUBSCRIPTIONS   │ handcut dovetails, no router       │  340k · 2h ago   │
│  ● rick beato   │ rex krueger · 1d · 22:03 · ✓       │  ──────────────  │
│  ● look mum no… │                                    │  why the 3340    │
│  ● rex krueger  │ the truth about quantum physics…   │  became the      │
│  ○ ai facts…    │ ai facts daily · 3h · ◆ likely AI  │  defining analog │
│                 │ starship + zsh: my 2026 shell      │  VCO chip…       │
│                 │ the cli corner · 1d · 12:47        │                  │
├─────────────────┴────────────────────────────────────┴──────────────────┤
│ ↵ play  a queue  p playlist  m mute  i inspect  / filter   ▓▓▓░░ 120/300 │
└─────────────────────────────────────────────────────────────────────────┘
```

Reading it: left nav rail (sections + subscriptions, `●` = has-new, `○` = muted),
centre feed grouped by day with `▎` marking selection and inline state badges
(`● new` · `▸ queued` · `✓ watched`-dim · `◆ likely AI`), right preview standing in for
the web app's hover-card, and a persistent status bar (keys · quota meter · sync).

### 7.1 Layout — three zones, responsive

Mirror the web app's three zones:
- **Nav rail (left):** sections up top (Feed, Recommended, Queue, Playlists, Liked, History),
  subscriptions listed below with per-channel state dots — exactly the web sidebar's structure.
- **Content (centre):** the feed, grouped by day, *bounded* (no infinite scroll).
- **Preview (right):** does the job of the web app's hover-preview — thumbnail, title, channel,
  metadata, and a Glamour-rendered description snippet for the selected item.

Responsive, and graceful degradation is a first-class requirement:
- **Wide:** all three panes.
- **Medium:** nav + content; preview toggles on demand or becomes a bottom panel.
- **Narrow:** single content pane; nav rail collapses to icons/overlay.

### 7.2 Content rendering — two modes, toggleable

- **Rich rows (default):** small thumbnail (if a graphics protocol is present) + two lines —
  title (bright/500) over `channel · age · duration · badges` (faint). Dense, scannable, best
  for triage.
- **Card grid:** larger thumbnails in a grid, closest to YouTube's homepage; lovelier on
  kitty/sixel terminals, fewer items per screen. Bind a key to switch modes.

### 7.3 State badges (what the web app hides)

`● new` · `▸ queued` · `✓ watched` (row dims rather than disappears — progress memory) ·
`◆ likely AI` (advisory, §5.2) · muted channels demoted/faint · provenance flag when present.
DeArrow-cleaned titles shown by default (de-clickbait), with a key to reveal the original.

### 7.4 Visual language ("lovely")

- **Palette:** warm near-black base, a single muted accent (a deliberately desaturated nod to
  YouTube red), and a small semantic set — teal=new, amber=queued, dimmed=spent. Use Lip Gloss
  adaptive colours so it respects the user's terminal theme + truecolor; ship 2–3 built-in
  themes. Never reproduce YouTube's white/red glare.
- **Hierarchy by weight/colour/dimming, not size** (one type size in a terminal): bright-bold
  titles, faint metadata, accent only where it carries meaning.
- **Selection** = left accent gutter + subtle background tint, not a full-row invert.
- **Chrome:** rounded borders (Lip Gloss), hairline separators, generous internal padding —
  restraint over decoration.
- **Designed states + micro-motion:** a considered first-run, a calm "all caught up" empty feed,
  a friendly offline banner, a gentle sync spinner, Harmonica-eased focus transitions, a smooth
  quota meter. Nothing bouncy.
- **Thumbnails:** kitty/iTerm/sixel where available; a *tuned* `chafa`/half-block fallback that
  still looks intentional; "off" as a config option. Cache under the store dir.
- **Glyphs:** sparing, meaningful (Nerd Font optional, ASCII fallback so it degrades cleanly).
- Visual kinship with `wwlog` so it reads as part of the same suite.

Status bar (persistent, honest system state): key hints · quota remaining · sync freshness ·
offline indicator · hidden-slop count.

Proposed keybindings (consistent with `wwlog`):

| Key | Action |
|---|---|
| `↑`/`↓` or `k`/`j` | Navigate |
| `tab` / `⇧tab` | Switch tabs |
| `enter` | Play in mpv |
| `A` | Play audio-only |
| `a` | Add to Queue |
| `p` | Add to playlist (picker) |
| `l` | Like / unlike |
| `m` | Mute channel |
| `i` | Inspect (LLM AI-slop analysis, cached) |
| `v` | Toggle view (rich rows ↔ card grid) |
| `t` | Reveal original title (undo DeArrow) |
| `/` | Filter current list |
| `s` | Cycle sort (new → duration → channel) |
| `r` | Sync / refresh |
| `x` | Hide (mark not-interested) |
| `o` | Open in browser |
| `y` | Yank URL to clipboard |
| `e` | Export (format picker) |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

### 7.6 Charmbracelet building blocks (what maps to what)

The design above is deliberately within reach of the Charm stack — each surface maps to a
specific library, which is why the mockup is realistic rather than aspirational:

- **Bubble Tea** — the runtime: Elm-style model/update/view event loop driving the whole app.
- **Lip Gloss** — styling and layout: the three-pane split (`JoinHorizontal`/`JoinVertical`),
  rounded window chrome, hairline borders, the selection gutter, and the adaptive palette
  (respects terminal light/dark + truecolor). This is where "lovely" mostly lives.
- **Bubbles** (components): `list` (feed + subscriptions), `table` (playlist/liked views),
  `viewport` (scrollable description pane), `textinput` (search/`/` filter), `spinner` (sync),
  `paginator`, and `help` + `key` (the status-bar hints and `?` overlay).
- **Glamour** — renders the video description as styled markdown in the preview pane.
- **Huh** — the first-run OAuth wizard, the config editor, and modal pickers (add-to-playlist).
- **Harmonica** — spring/easing for pane-focus transitions and the quota meter. Keep it subtle.
- **charmbracelet/log** — structured logging.
- **VHS** (Charm) — script the README demo GIF, exactly like the `wwlog` demo.

Not Charm, and worth flagging: **thumbnails**. Charm has no image primitive — rendering goes
through the terminal graphics protocol (kitty/iTerm/sixel) via a Go image lib, with a `chafa`
shell-out / half-block fallback (§7.4). Treat it as an isolated adapter, not a core concern.

---

## 8. Config (`~/.config/unspool/config.toml`)

```toml
store_dir              = ""            # local store path (default: alongside config)
max_resolution         = 1080
audio_only_default     = false
playback_detached      = true
thumbnails             = "auto"        # "auto" | "chafa" | "halfblock" | "off"
theme                  = "warm-dark"    # built-in themes; adaptive to terminal truecolor
view_mode              = "rows"          # "rows" | "grid"
dearrow                = true            # show de-clickbaited titles/thumbnails by default
cookies_from_browser   = ""            # playback auth only (age-gated/members); "" | "firefox" | "chrome" | "safari"
sponsorblock           = ["sponsor", "selfpromo", "interaction"]

[queue]
mirror                 = true          # v1: keep Queue synced to a real playlist
mirror_playlist_id     = ""            # blank = auto-create "▶ unspool Queue"; or point at an existing one

[recommendations]
enabled                = true          # synthesised from subs + watch log (§5.8)

[filters]
hide_shorts            = true          # UULF source; leave on
ai_score_threshold     = 0.7           # dim/hide feed items scoring above this (0 = off)
ai_autohide            = false         # false = badge+dim; true = hide outright
show_synthetic_flag    = true          # surface status.containsSyntheticMedia

[classifier]
# Model-agnostic shell-out. Empty = built-in metadata heuristics only.
transcript_command     = ""            # tier 1: receives transcript text on stdin, returns score/verdict JSON
inspect_command        = ""            # tier 2: receives video URL as $1, returns verdict + suspected tools (on-demand `i`)
auto_inspect_new_channels = false      # run tier 1 automatically on first sight of an unknown channel
cache_verdicts         = true          # cache per-video-ID so `i` never re-pays
```

---

## 9. Distribution

- `goreleaser` cross-builds (darwin/linux, arm64/amd64), GitHub Releases, and a formula in
  the **existing** tap `ali5ter/homebrew-tap` → `brew install ali5ter/tap/unspool`. Same
  goreleaser + tap pipeline as `wwlog`; just add an `unspool.rb` formula to the tap.
- `go install`, curl-tarball, and `brew install` install paths in README.
- Runtime deps (`mpv`, `yt-dlp`) documented with per-platform install one-liners; the tool
  detects them at startup and prints actionable guidance if missing.

---

## 10. Decisions (all locked)

- Name: **`unspool`** — module `github.com/ali5ter/unspool`, binary `unspool`, formula in the existing `ali5ter/homebrew-tap`.
- Data access: **API-only for v1** (§6.0); cookie read-path is v2.
- Store: **plain JSON**, `wwlog`-style (§6.2).
- Recommendations: **synthesised** from own data, in v1 (§5.8).
- Queue: **auto-mirrored** to a real playlist, in v1 (§5.4).
- AI filtering: metadata heuristics + channel-mute **and** the LLM inspect hook (`i`) — all in v1 (§5.2).
- Thumbnails: **in v1** with graceful degradation (§7).

Nothing outstanding — ready to scaffold. Recommended first step for Claude Code: `go mod init
github.com/ali5ter/unspool`, then build M1 (§11).

---

## 11. Suggested milestones (all API-only)

- **M1 — Read-only feed.** OAuth login (keychain, loopback flow) + `docs/SETUP.md`,
  subscription resolve, `UULF` RSS feed, JSON store, Feed tab, mpv playback, watch log.
  *The core daily driver.*
- **M2 — Curation.** Queue with **auto-mirror**, channel mute, Playlists tab
  (view/create/add/remove/reorder), Liked view, like/unlike. *Delivers "manage".*
- **M3 — Filtering, recs, search.** AI metadata heuristics + badges + the LLM inspect hook
  (`i`) with transcript tier, synthesised **Recommended** tab (§5.8), rationed `search.list`,
  SponsorBlock, audio-only. *Delivers the anti-slop + discovery pitch.*
- **M4 — Polish + ship.** Thumbnails with graceful degradation, `--json`/`--export`/`--sync`/
  `--offline` pipeline, full config surface, goreleaser + Homebrew tap. *Ship v1.*
- **v2 (post-ship) — Cookie read-path.** Behind a flag: yt-dlp `--cookies-from-browser`
  reader for the real subscription feed, Watch Later, history, and `:ytrec` recommendations,
  slotting into the §6.2 data-source interface. Opt-in; carries the §2.4 fragility/ToS caveats.

---

## 12. Acceptance criteria (v1 / M1–M2)

- Fresh install → `unspool --login` → feed populates with **no Shorts present**.
- New uploads from subscriptions appear after `r`/`--sync` without exceeding a few hundred
  quota units for a typical (~150–300 sub) account.
- `enter` plays the selected video in mpv; the launch appears in History.
- Muting a channel removes it from the feed and survives restart.
- Adding a video to a playlist is reflected in the YouTube app.
- Adding to the Queue creates/updates the mirrored "▶ unspool Queue" playlist on YouTube.
- The Recommended tab shows subs-derived suggestions each with a plain-language "why".
- Thumbnails render (or degrade cleanly) in the target terminal.
- `unspool feed --json | jq …` works with the TUI closed.
- Every "hidden"/"filtered" item is recoverable; the tool never hard-deletes YouTube data.
