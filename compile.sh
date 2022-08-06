#!/bin/sh

go build -o binaries/PanelExplorer
GOOS=darwin GOARCH=arm64 go build -o binaries/PanelExplorer.Mac-ARM
GOOS=windows GOARCH=amd64 go build -o binaries/PanelExplorer.exe

cd binaries

zip PanelExplorer.Mac.zip PanelExplorer PanelExplorer.Mac-ARM
zip PanelExplorer.Win.zip PanelExplorer.exe

cd ..