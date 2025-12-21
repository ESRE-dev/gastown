#!/bin/bash
# Install gt locally with proper version info
# Run from any gastown rig directory

set -e

# Get version from source
VERSION=$(grep 'Version.*=' internal/cmd/version.go | head -1 | cut -d'"' -f2)
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "Building gt v${VERSION} (${COMMIT})..."

go build -ldflags="-X github.com/steveyegge/gastown/internal/cmd.Version=${VERSION} \
  -X github.com/steveyegge/gastown/internal/cmd.GitCommit=${COMMIT} \
  -X github.com/steveyegge/gastown/internal/cmd.BuildTime=${BUILD_TIME}" \
  -o /Users/stevey/gt/gt ./cmd/gt

# Ensure symlink exists
if [ ! -L ~/.local/bin/gt ]; then
  ln -sf /Users/stevey/gt/gt ~/.local/bin/gt
  echo "Created symlink ~/.local/bin/gt → /Users/stevey/gt/gt"
fi

echo "Installed:"
/Users/stevey/gt/gt version
