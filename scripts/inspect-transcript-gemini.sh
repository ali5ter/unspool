#!/usr/bin/env bash
#
# @file inspect-transcript-gemini.sh
# @brief unspool tier-1 transcript_command: judges a video's auto-generated transcript for AI-slop signals.
# @description
#   A concrete, working example of unspool's classifier.transcript_command
#   hook (PRD.md §5.2.4 tier 1, config.toml's [classifier] section).
#   unspool runs this opportunistically for the newest video of each
#   brand-new channel discovered during a sync (see
#   internal/feed/feed.go's runTier1) and pipes the video's transcript text
#   (already fetched by unspool via yt-dlp) on stdin — this script never
#   fetches anything itself, unlike inspect-gemini.sh (tier 2), which is
#   given a video URL and does its own fetching. Forces Gemini's response
#   into the same JSON shape unspool's classifier package expects on
#   stdout: {"likely_ai": bool, "score": number, "reasoning": string,
#   "suspected_tools": [string]}.
#
#   Deliberately prints nothing but that JSON on stdout — unspool parses
#   stdout directly, so this intentionally skips the usual pfb-styled
#   terminal output conventions (this is a machine-facing hook, not an
#   interactive script); any diagnostic output goes to stderr instead.
#
# @author Alister Lewis-Bowen <alister@lewis-bowen.org>
# @version 1.0.0
# @date 2026-07-25
# @license MIT
# @usage
#   In config.toml:
#     [classifier]
#     transcript_command = "/absolute/path/to/inspect-transcript-gemini.sh"
#     auto_inspect_new_channels = true
#   Runs automatically, at most classifier.maxTier1PerSync videos per sync,
#   for the newest video of each newly-discovered channel.
# @dependencies curl, jq, a Gemini API key (https://aistudio.google.com/apikey)
# @exit_codes 0=success, 1=missing dependency or API key, 2=API call failed
#
# @example
#   GEMINI_API_KEY=... ./inspect-transcript-gemini.sh < transcript.txt

set -euo pipefail

# Same .env fallback as inspect-gemini.sh (tier 2) — see that script's
# comment for why: a plain `export` only lasts for the shell session it
# was run in.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "${GEMINI_API_KEY:-}" && -f "$script_dir/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$script_dir/.env"
  set +a
fi

model="${GEMINI_MODEL:-gemini-2.5-flash}"
api_key="${GEMINI_API_KEY:?Set GEMINI_API_KEY, either exported or in scripts/.env (see scripts/.env.template) — get one at https://aistudio.google.com/apikey}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || {
    echo "inspect-transcript-gemini.sh: missing dependency: $bin" >&2
    exit 1
  }
done

transcript="$(cat)"
if [[ -z "$transcript" ]]; then
  echo "inspect-transcript-gemini.sh: empty transcript on stdin" >&2
  exit 2
fi

# Text-only judgment (no video/audio to inspect at this tier, unlike tier
# 2) — kept plain, not clickbait-y itself, same reasoning as inspect-gemini.sh.
prompt='Judge, from this auto-generated video transcript alone, whether the
video sounds AI-generated: an AI-written script, AI voiceover reading
templated/robotic phrasing, or other textual signs of AI-slop content
(e.g. generic listicle structure, repetitive filler, no natural speech
disfluencies at all). A transcript alone is weak evidence either way —
say so plainly in your reasoning, and keep your score modest unless the
text itself is a strong signal (e.g. it explicitly states the content is
AI-generated). Most transcripts are from real human speech — do not
default to true.'

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
  --arg transcript "$transcript" \
  --arg prompt "$prompt" \
  --argjson schema "$response_schema" \
  '{
    contents: [{parts: [
      {text: "Transcript:\n" + $transcript},
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
  echo "inspect-transcript-gemini.sh: Gemini API returned HTTP $http_code: $body" >&2
  exit 2
fi

jq -r '.candidates[0].content.parts[0].text' <<<"$body"
