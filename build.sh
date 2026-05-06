#!/bin/bash

# SniShaper Linux Build Script

set -e

echo "Building SniShaper CLI..."

# Create dist directory
mkdir -p dist

# Build binary
go build -o dist/snishaper main.go

# Copy default rules
cp -r rules dist/

echo "Build complete!"
echo "Binary: dist/snishaper"
echo "Rules: dist/rules/"
