#!/usr/bin/env bash
#
# @file inspect-gemini.sh
# @brief unspool tier-2 inspect_command: asks Gemini whether a YouTube video looks AI-generated.
# @description
#   A concrete, working example of unspool's classifier.inspect_command hook
#   (see PRD.md §5.2.4, config.toml's [classifier] section). Gemini can
#   ingest a YouTube URL directly (no download/transcript step needed) —
#   this sends that URL plus a judgment prompt, and forces Gemini's
#   response into the exact JSON shape unspool's classifier package expects
#   on stdout: {"likely_ai": bool, "score": number, "reasoning": string,
#   "suspected_tools": [string]}.
#
#   Deliberately prints nothing but that JSON on stdout — unspool parses
#   stdout directly, so this intentionally skips the usual pfb-styled
#   terminal output conventions (this is a machine-facing hook, not an
#   interactive script); any diagnostic output goes to stderr instead.
#
# @author Alister Lewis-Bowen <alister@lewis-bowen.org>
# @version 1.1.0
# @date 2026-07-24
# @license MIT
# @usage
#   In config.toml:
#     [classifier]
#     inspect_command = "/absolute/path/to/inspect-gemini.sh"
#   Then press `i` on a selected video in the unspool TUI.
# @dependencies curl, jq, yt-dlp, a Gemini API key (https://aistudio.google.com/apikey)
# @exit_codes 0=success, 1=missing dependency or API key, 2=API call failed
#
# @example
#   GEMINI_API_KEY=... ./inspect-gemini.sh "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

set -euo pipefail

# Fall back to a .env file next to this script when GEMINI_API_KEY isn't
# already in the environment — a plain `export` only lasts for the shell
# session it was run in, which caused two separate "it's broken" reports
# in practice (a fresh terminal, or unspool launched from one that never
# got the export, silently had no key by the time this script ran). An
# already-exported value still wins if present; .env is just the fallback.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "${GEMINI_API_KEY:-}" && -f "$script_dir/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$script_dir/.env"
  set +a
fi

video_url="${1:?usage: inspect-gemini.sh <video-url>}"
model="${GEMINI_MODEL:-gemini-2.5-flash}"
api_key="${GEMINI_API_KEY:?Set GEMINI_API_KEY, either exported or in scripts/.env (see scripts/.env.template) — get one at https://aistudio.google.com/apikey}"

for bin in curl jq yt-dlp; do
  command -v "$bin" >/dev/null 2>&1 || {
    echo "inspect-gemini.sh: missing dependency: $bin" >&2
    exit 1
  }
done

# Real-world finding (2026-07-24): a video's actual description read "All
# clips are fully generated for entertainment by Rocky Pictures" — a plain
# creator disclosure — and an earlier version of this prompt still missed
# it, because it only ever asked Gemini to watch the video, never to read
# its title/description. Whether Gemini's own YouTube-URL ingestion
# surfaces that text on its own isn't documented/guaranteed, so fetch it
# explicitly via yt-dlp and hand it over as plain text rather than relying
# on that.
video_json=$(yt-dlp -j --skip-download "$video_url" 2>/dev/null) || {
  echo "inspect-gemini.sh: yt-dlp failed to fetch video metadata" >&2
  exit 2
}
video_title=$(jq -r '.title // ""' <<<"$video_json")
video_description=$(jq -r '.description // ""' <<<"$video_json")

# Kept plain, not clickbait-y itself — asking an AI to "catch AI slop" is
# already a low-precision exercise (PRD §5.2); an unbiased prompt matters.
prompt='Judge whether this YouTube video is AI-generated: AI voiceover/
narration, an AI-written script, AI-generated imagery or an AI avatar, or
a faceless channel that appears AI-assembled end to end. You are given
both the video itself and its own title/description as written by the
uploader — weigh both. Creators sometimes explicitly disclose AI/synthetic
generation in the title or description (phrases like "AI generated",
"fully generated", "made with AI", "AI voiceover") even when the footage
itself looks and sounds convincingly real to a purely visual/audio
judgment — a plain-text disclosure like that should weigh heavily
regardless of how real the footage looks. Most videos are NOT
AI-generated — do not default to true just because a video is low-budget,
narrated, or a compilation. Explain specifically what tipped you off, if
anything (and say plainly if it was the description text rather than
anything visual/audio), and name any AI tools you suspect were used, if
you can tell. Be honest about uncertainty.'

response_schema='{
  "type": "OBJECT",
  "properties": {
    "likely_ai": {"type": "BOOLEAN"},
    "score": {"type": "NUMBER", "description": "0-1 confidence that this is AI-generated"},
    "reasoning": {"type": "STRING"},
    "suspected_tools": {"type": "ARRAY", "items": {"type": "STRING"}}
  },
  "required": ["likely_ai", "reasoning"]
}'

payload=$(jq -n \
  --arg url "$video_url" \
  --arg title "$video_title" \
  --arg description "$video_description" \
  --arg prompt "$prompt" \
  --argjson schema "$response_schema" \
  '{
    contents: [{parts: [
      {file_data: {file_uri: $url}},
      {text: "Video title: " + $title + "\n\nVideo description:\n" + $description},
      {text: $prompt}
    ]}],
    generationConfig: {responseMimeType: "application/json", responseSchema: $schema}
  }')

http_response=$(curl -sS -w '\n%{http_code}' \
  "https://generativelanguage.googleapis.com/v1beta/models/${model}:generateContent?key=${api_key}" \
  -H 'Content-Type: application/json' \
  -d "$payload")

http_code=$(tail -n1 <<<"$http_response")
body=$(sed '$d' <<<"$http_response")

if [[ "$http_code" != "200" ]]; then
  echo "inspect-gemini.sh: Gemini API returned HTTP $http_code: $body" >&2
  exit 2
fi

# generationConfig.responseMimeType=application/json forces the model's own
# text response to already be the JSON verdict — no further parsing needed.
jq -r '.candidates[0].content.parts[0].text' <<<"$body"
