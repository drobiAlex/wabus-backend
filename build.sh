#!/bin/bash

set -e

APP_NAME="wabus"
OUTPUT_DIR="./bin"

mkdir -p "$OUTPUT_DIR"

echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o "$OUTPUT_DIR/${APP_NAME}-linux-amd64" ./cmd/wabus

echo "Build complete: $OUTPUT_DIR/${APP_NAME}-linux-amd64"
