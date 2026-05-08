#!/bin/bash

# SniShaper Linux Build Script

set -e

echo "Building SniShaper CLI for Linux..."

# Create dist directory
mkdir -p dist

# Build binary
GOOS=linux GOARCH=amd64 go build -o dist/snishaper .

# Copy default rules
cp -r rules dist/

# Copy tun2socks binary if exists
if [ -f "tun2socks" ]; then
    cp tun2socks dist/
    chmod +x dist/tun2socks
    echo "✓ tun2socks copied to dist/"
fi

echo ""
echo "Build complete!"
echo "Binary: dist/snishaper"
echo "Rules: dist/rules/"
if [ -f "dist/tun2socks" ]; then
    echo "TUN: dist/tun2socks"
fi
echo ""
echo "To run: cd dist && ./snishaper"
