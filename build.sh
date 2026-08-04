#!/usr/bin/env bash
# Build the app-template wasm bundle into dist/.
#
# Usage:
#   ./build.sh
#
# Produces dist/{app.wasm, wasm_exec.js, index.html}. Serve dist/ over HTTP
# (a file:// open will not instantiate the wasm), e.g.:
#
#   ./build.sh && (cd dist && python3 -m http.server 8080)
#
# CGO is disabled (pure Go). Only main.go carries the js && wasm build tag;
# the ViewModel + View are native-testable.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dist="$here/dist"
mkdir -p "$dist"

echo "==> building app.wasm"
( cd "$here" && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o "$dist/app.wasm" . )

echo "==> copying wasm_exec.js"
goroot="$(go env GOROOT)"
if [ -f "$goroot/lib/wasm/wasm_exec.js" ]; then
  cp "$goroot/lib/wasm/wasm_exec.js" "$dist/wasm_exec.js"
elif [ -f "$goroot/misc/wasm/wasm_exec.js" ]; then
  cp "$goroot/misc/wasm/wasm_exec.js" "$dist/wasm_exec.js"
else
  echo "error: wasm_exec.js not found under $goroot (lib/wasm or misc/wasm)" >&2
  exit 1
fi

echo "==> copying index.html"
cp "$here/index.html" "$dist/index.html"

echo "==> done: $dist"
ls -la "$dist"
