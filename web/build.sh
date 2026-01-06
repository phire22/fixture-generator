#!/bin/bash
set -e

cd "$(dirname "$0")"

# Copy the Go WASM support file
rm -f wasm_exec.js
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .

# Build the WASM binary
GOOS=js GOARCH=wasm go build -o main.wasm main.go

echo "Build complete! Files:"
echo "  - main.wasm"
echo "  - wasm_exec.js"
echo "  - index.html"
echo ""
echo "To test locally, run:"
echo "  python3 -m http.server 8080"
echo "Then open http://localhost:8080"
