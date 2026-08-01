#!/usr/bin/env bash
# @file install-deps.sh
# @brief Install unspool's runtime dependencies: mpv, yt-dlp, and chafa.
# @description
#   Codifies the manual "brew install mpv yt-dlp chafa" step from the README's Quick start —
#   mpv/yt-dlp drive playback (internal/playback), chafa renders preview-pane thumbnails
#   (internal/thumbnail). Neither is required for unspool to start (see issue #8): a missing
#   dependency only disables the feature it gates and prints a warning at launch. Homebrew
#   installs of unspool itself already pull these in as formula dependencies
#   (.goreleaser.yml) — this script is for `go install`/build-from-source installs, or anyone
#   who skipped them.
#
#   Idempotent: already-installed tools are reported and skipped, not reinstalled.
#
# @author Alister Lewis-Bowen <alister@lewis-bowen.org>
# @version 1.0.0
# @date 2026-08-01
# @license MIT
#
# @usage ./scripts/install-deps.sh
#
# @dependencies pfb, and one of: Homebrew (macOS), apt or pacman (Linux)
#
# @exit 0 every dependency present (already installed, or installed by this run)
# @exit 1 missing pfb, or no supported package manager found
set -euo pipefail

type pfb >/dev/null 2>&1 || {
    echo "error: pfb is required." >&2
    echo "  macOS: brew tap ali5ter/pfb && brew install pfb" >&2
    exit 1
}

DEPS=(mpv yt-dlp chafa)

# @description Installs whichever of DEPS aren't already on PATH, via the first supported
#   package manager found for the current OS.
# @return 0 on success; exits 1 if no supported package manager is found
# @example install_missing
install_missing() {
    local missing=()
    local dep
    for dep in "${DEPS[@]}"; do
        if type "$dep" >/dev/null 2>&1; then
            pfb success "$dep already installed"
        else
            missing+=("$dep")
        fi
    done

    if [[ ${#missing[@]} -eq 0 ]]; then
        pfb success "All runtime dependencies already installed"
        return 0
    fi

    pfb heading "Installing: ${missing[*]}" "📦"
    if [[ "$OSTYPE" == "darwin"* ]] && type brew >/dev/null 2>&1; then
        brew install "${missing[@]}"
    elif type apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y "${missing[@]}"
    elif type pacman >/dev/null 2>&1; then
        sudo pacman -S --needed "${missing[@]}"
    else
        pfb error "No supported package manager found (Homebrew, apt, or pacman)"
        pfb suggestion "Install manually: https://mpv.io, https://github.com/yt-dlp/yt-dlp, https://hpjansson.org/chafa/"
        exit 1
    fi
    pfb success "Installed: ${missing[*]}"
}

main() {
    pfb heading "unspool — runtime dependencies" "🔧"
    install_missing
}

main "$@"
