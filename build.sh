#!/bin/bash

# This script builds the application for Linux AMD64.

echo "Building for Linux AMD64..."

# Set the target OS and architecture
export GOOS=linux
export GOARCH=amd64

# Build the application
go build -o proxy-filter-linux ./cmd/proxy-filter/main.go

# Unset the environment variables (optional, good practice)
unset GOOS
unset GOARCH

echo "Build complete! Executable: proxy-filter-linux"
