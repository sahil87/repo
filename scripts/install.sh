#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/hop"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/hop "$DEST"
echo "installed: $DEST"
