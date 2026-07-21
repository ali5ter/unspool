#!/usr/bin/env bash

# @file run_vhs.sh
# @brief Generate demo GIFs from .tape files using vhs
# @description
#   Builds the unspool binary (the tapes run it as ../unspool relative to
#   this directory), then runs one or all .tape files through vhs.
# @ref https://github.com/charmbracelet/vhs
# @usage ./run_vhs.sh [tape-file|all]
# @example
#   ./run_vhs.sh                  # regenerate every demo
#   ./run_vhs.sh unspool.tape     # regenerate just this one
# @dependencies vhs, go
# @exit_codes 0=success, 1=vhs not installed, 1=build failed

tape=${1:-"all"}

type vhs &>/dev/null || {
  echo "vhs is not installed. Refer to https://github.com/charmbracelet/vhs for installation instructions."
  exit 1
}

# https://github.com/charmbracelet/vhs/issues/419
unset PROMPT_COMMAND

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
(cd "$script_dir/.." && go build -o unspool .) || {
  echo "go build failed."
  exit 1
}

cd "$script_dir" || exit 1

if [[ "$tape" != "all" ]]; then
  vhs "$tape"
else
  for tape in *.tape; do
    # Skip the sourced vhs configuration file
    [[ "$tape" == "config.tape" ]] && continue
    vhs "$tape"
  done
fi
