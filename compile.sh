#!/bin/sh

go build -o binaries/PanelExplorer
GOOS=windows GOARCH=amd64 go build -o binaries/PanelExplorer.exe

cd binaries

zip PanelExplorer.Mac.zip PanelExplorer
zip PanelExplorer.Win.zip PanelExplorer.exe

cd ..