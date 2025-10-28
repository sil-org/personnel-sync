#!/usr/bin/env bash

# Exit script with error if any step fails.
set -e

# Echo out all commands for monitoring progress
set -x

# Build all the things
go build -C src -ldflags="-s -w" -o bin/bootstrap personnelsync.go
cp src/*.json src/bin
