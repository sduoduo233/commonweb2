#!/bin/bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-linux-amd64
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-linux-386
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-linux-arm64

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-windows-amd64.exe
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-windows-386.exe
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/commonweb2-windows-arm64.exe