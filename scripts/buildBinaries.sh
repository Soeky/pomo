#!/bin/bash

# Für Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o pomo-linux-amd64

# Für macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o pomo-darwin-amd64

# Für Windows
GOOS=windows GOARCH=amd64 go build -o pomo-windows-amd64.exe
