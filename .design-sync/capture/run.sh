#!/bin/sh
# Renders real Svelte components against fixtures (jsdom) and writes their HTML into $1.
# The capture files need web/node_modules, so they run from a temporary folder under web/src.
set -e
out=$(cd "$1" && pwd)
repo=$(cd "$(dirname "$0")/../.." && pwd)
tmp="$repo/web/src/ds-capture-tmp"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT
for f in "$repo"/.design-sync/capture/*.capture.ts; do cp "$f" "$tmp/$(basename "$f" .capture.ts).test.ts"; done
cd "$repo/web" && DS_CAPTURE_OUT="$out" npx vitest run src/ds-capture-tmp
