#!/bin/bash
# Source this file to add the local Go toolchain to your PATH:
#   source source-me.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export GOROOT="$SCRIPT_DIR/.tools/go"
export PATH="$GOROOT/bin:$PATH"

echo "Go $(go version | awk '{print $3}') activated from .tools/"
