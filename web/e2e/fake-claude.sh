#!/bin/sh
# Stand-in for the claude CLI: prints how it was started, then echoes input.
echo "FAKE-CLAUDE"
for a in "$@"; do echo "arg=[$a]"; done
echo "cwd=$(pwd)"
while IFS= read -r line; do echo "echo:$line"; done
