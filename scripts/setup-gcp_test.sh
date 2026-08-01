#!/usr/bin/env bash
# @file setup-gcp_test.sh
# @brief Unit tests for setup-gcp.sh's pure helper functions.
# @description
#   Sources setup-gcp.sh (guarded so main() only runs when executed directly — see
#   setup-gcp.sh) and exercises resolve_project/client_secret_path directly against known
#   inputs. Needs no gcloud, pfb, or network access — covers exactly the two questions this
#   script is repeatedly asked about (see issue #9): "which project?" and "where does the
#   client secret go?". Not part of `go test ./...`; run directly or wire into a future
#   non-Go CI step.
#
# @author Alister Lewis-Bowen <alister@lewis-bowen.org>
# @version 1.0.0
# @date 2026-08-01
# @license MIT
#
# @usage ./scripts/setup-gcp_test.sh
#
# @dependencies bash
#
# @exit 0 every assertion passed
# @exit 1 an assertion failed
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=setup-gcp.sh
source ./setup-gcp.sh

failures=0

# @description Asserts got == want, printing a PASS/FAIL line either way and tallying failures.
# @param $1 label — human-readable name for the assertion
# @param $2 got — actual value
# @param $3 want — expected value
# @example assert_eq "resolve_project(explicit arg)" "$(resolve_project my-project)" "my-project"
assert_eq() {
    local label="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        echo "PASS: $label"
    else
        echo "FAIL: $label — got '$got', want '$want'"
        failures=$((failures + 1))
    fi
}

main() {
    # resolve_project: an explicit ID is returned as-is, with no gcloud call
    # (gcloud isn't even on this test's PATH — a call here would abort the
    # whole script under set -e, which is itself part of what's covered).
    assert_eq "resolve_project(explicit arg)" "$(resolve_project my-project)" "my-project"

    # client_secret_path must always agree with config.DefaultClientSecretFile
    # (config/config.go) — the two are hand-maintained in separate languages
    # and have no shared source of truth, so a drift between them would only
    # ever surface as a confusing "wrong path" bug report.
    assert_eq "client_secret_path(darwin)" \
        "$(OSTYPE=darwin23 client_secret_path)" \
        "$HOME/Library/Application Support/unspool/client_secret.json"
    assert_eq "client_secret_path(linux)" \
        "$(OSTYPE=linux-gnu client_secret_path)" \
        "$HOME/.config/unspool/client_secret.json"

    if [[ "$failures" -gt 0 ]]; then
        echo "$failures assertion(s) failed"
        exit 1
    fi
    echo "All assertions passed"
}

main "$@"
