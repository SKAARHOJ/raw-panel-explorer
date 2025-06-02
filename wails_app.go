package main

import (
	"context"
	"fmt"

	"github.com/pkg/browser"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// URL
func (a *App) Url() string {
	return fmt.Sprintf("http://localhost:%d", *WebServerPort)
}

func (a *App) OpenURLInBrowser(url string) error {
	fmt.Println("OpenURLInBrowser called with:", url)
	return browser.OpenURL(url)
}
