#!/bin/sh
set -eu
PRESS_TASK_ROOT=/Users/cathrynlavery/printing-press/.runstate/printing-press-c52790a1/runs/20260921-133106-1fbd7f53
exec env -i PATH="$PATH" HOME="$HOME" TMPDIR="${TMPDIR:-/tmp}" GOCACHE="${GOCACHE}" GOMODCACHE="${GOMODCACHE}" GOWORK=off SENDFOX_HOME="$PRESS_TASK_ROOT/isolated" SENDFOX_CONFIG="$PRESS_TASK_ROOT/isolated/empty.toml" SENDFOX_BASE_URL=http://127.0.0.1:9 PRINTING_PRESS_HOME=/Users/cathrynlavery/printing-press "$@"
