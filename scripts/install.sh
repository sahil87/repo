#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/repo"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/repo "$DEST"
echo "installed: $DEST"
