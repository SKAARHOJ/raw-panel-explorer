//go:build (linux && arm64) || windows

package main

// Dummy implementation of the GUI interface for Linux/ARM64 platforms.
type FyneGui struct{}

func (gui *FyneGui) Create(WebServerPort uint32, appName, appFriendlyName string) uint32 {
	return WebServerPort
}

func (gui *FyneGui) ShowAndRun() {
}

func checkIfPackaged() bool {
	return false
}
