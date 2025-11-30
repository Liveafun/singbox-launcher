#!/bin/bash

# Build script for macOS (Darwin)

set -e

cd "$(dirname "$0")/.."

echo ""
echo "========================================"
echo "  Building Sing-Box Launcher (macOS)"
echo "========================================"
echo ""

echo "=== Tidying Go modules ==="
go mod tidy

echo ""
echo "=== Setting build environment ==="
export CGO_ENABLED=1
export GOOS=darwin
export GOARCH=amd64

# Determine output filename
BASE_NAME="singbox-launcher"
EXTENSION=""
OUTPUT_FILENAME="${BASE_NAME}${EXTENSION}"
COUNTER=0

while [ -f "$OUTPUT_FILENAME" ]; do
    COUNTER=$((COUNTER + 1))
    OUTPUT_FILENAME="${BASE_NAME}-${COUNTER}${EXTENSION}"
done

echo "Using output file: $OUTPUT_FILENAME"

echo ""
echo "=== Starting Build ==="
go build -buildvcs=false -ldflags="-s -w" -o "$OUTPUT_FILENAME"

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "  Build completed successfully!"
    echo "  Output: $OUTPUT_FILENAME"
    echo "========================================"
else
    echo ""
    echo "!!! Build failed !!!"
    exit 1
fi

