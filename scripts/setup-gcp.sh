#!/usr/bin/env bash
# @file setup-gcp.sh
# @brief Create/select a Google Cloud project and enable the YouTube Data API v3 for unspool.
# @description
#   Codifies the scriptable half of docs/SETUP.md: creating (or reusing) a GCP project and
#   enabling the YouTube Data API v3 on it. OAuth consent screen configuration and Desktop-app
#   OAuth client ID creation have no stable gcloud equivalent as of this writing and must be
#   done in the Cloud Console — this script prints the direct URL for that step and stops
#   there rather than guessing at a command that may not exist.
#
#   Idempotent: safe to re-run against an existing project.
#
# @author Alister Lewis-Bowen <alister@lewis-bowen.org>
# @version 1.1.0
# @date 2026-08-01
# @license MIT
#
# @usage ./scripts/setup-gcp.sh [project-id]
#   project-id   Optional. Defaults to "unspool-<random-suffix>" for a new project,
#                or prompts to reuse the current gcloud project if one is set.
#
# @dependencies gcloud, pfb
#
# @exit 0 API enabled successfully; manual OAuth client step printed
# @exit 1 Missing dependency or gcloud command failure
#
# Sourceable: dependency checks and side effects only run inside main(),
# invoked at the bottom guarded to fire only when executed directly — see
# scripts/setup-gcp_test.sh, which sources this file to unit-test
# resolve_project/client_secret_path without gcloud, pfb, or network access.
set -euo pipefail

API="youtube.googleapis.com"

# @description Resolves the GCP project to use: explicit arg, else current gcloud config,
#   else a freshly created project.
# @param $1 requested_id — optional project ID passed on the command line
# @return Prints the resolved project ID to stdout
# @example resolve_project "my-project"
resolve_project() {
    local requested_id="${1:-}"

    if [[ -n "$requested_id" ]]; then
        echo "$requested_id"
        return 0
    fi

    local current
    current="$(gcloud config get-value project 2>/dev/null || true)"
    if [[ -n "$current" && "$current" != "(unset)" ]]; then
        if pfb confirm "Use current gcloud project '$current' for unspool?" yes; then
            echo "$current"
            return 0
        fi
    fi

    local new_id="unspool-$(date +%s | tail -c 6)"
    echo "$new_id"
}

# @description Prints unspool's expected client_secret.json path, matching
#   os.UserConfigDir() in config/config.go (Application Support on macOS, XDG on Linux).
# @return Prints the path to stdout
# @example client_secret_path
client_secret_path() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "$HOME/Library/Application Support/unspool/client_secret.json"
    else
        echo "$HOME/.config/unspool/client_secret.json"
    fi
}

main() {
    type pfb >/dev/null 2>&1 || {
        echo "error: pfb is required." >&2
        echo "  macOS: brew tap ali5ter/pfb && brew install pfb" >&2
        exit 1
    }
    type gcloud >/dev/null 2>&1 || {
        pfb error "gcloud is required — install the Google Cloud SDK first"
        pfb suggestion "https://cloud.google.com/sdk/docs/install"
        exit 1
    }

    pfb heading "unspool — Google Cloud setup" "☁️"

    local project_id
    project_id="$(resolve_project "${1:-}")"

    if gcloud projects describe "$project_id" >/dev/null 2>&1; then
        pfb info "Project '$project_id' already exists — reusing it"
    else
        pfb heading "Creating project" "🆕"
        gcloud projects create "$project_id" --name="unspool"
        pfb success "Project '$project_id' created"
    fi

    gcloud config set project "$project_id" >/dev/null
    pfb success "Active gcloud project set to '$project_id'"

    pfb heading "Enabling YouTube Data API v3" "🔌"
    gcloud services enable "$API" --project="$project_id"
    pfb success "$API enabled"

    pfb heading "Manual step required" "🖱️"
    pfb subheading "Google has no scriptable path for OAuth consent screen + client creation"
    pfb info "1. Open the OAuth consent screen and configure it (External, add your account as a test user):"
    pfb suggestion "https://console.cloud.google.com/auth/overview?project=$project_id"
    pfb info "2. Create an OAuth client ID of type 'Desktop app':"
    pfb suggestion "https://console.cloud.google.com/auth/clients?project=$project_id"
    pfb info "3. Download the client JSON and save it to:"
    pfb suggestion "  $(client_secret_path)"
    pfb info "4. Then run: unspool --login"
}

# Only run main when executed directly — sourcing (e.g. from
# scripts/setup-gcp_test.sh) loads the functions above with no side
# effects, so they're unit-testable without gcloud/pfb/network access.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
