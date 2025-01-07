/*
Distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. MIT License
*/
package main

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"time"

	rwp "github.com/SKAARHOJ/rawpanel-lib/ibeam_rawpanel"
	log "github.com/s00500/env_logger"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var PanelToSystemMessages *bool
var writeTopologiesToFiles *bool
var AggressiveQuery *bool
var Dark *bool

var triggerRecording = &TriggerRecording{}
var RecordTriggers *string
var appLaunchTime = time.Now()

const (
	appName         = "raw-panel-explorer"
	appFriendlyName = "Raw Panel Explorer"
)

//go:embed all:frontend/dist
var assets embed.FS

var WebServerPort *int

func main() {

	// Welcome message!
	fmt.Println("Welcome to Raw Panel Explorer made by Kasper Skaarhoj (c) 2022-2024")
	fmt.Println("Opens a Web Browser on localhost:8051 to explore the topology interactively.")
	fmt.Println("usage: [options] [panelIP:port] [Shadow panelIP:port]")
	fmt.Println("-h for help")
	fmt.Println()

	// Setting up and parsing command line parameters
	//binPanel := flag.Bool("binPanel", false, "Works with the panel in binary mode")
	PanelToSystemMessages = flag.Bool("panelToSystemMessages", false, "If set, you will see panel to system messages written to the console")
	writeTopologiesToFiles = flag.Bool("writeTopologiesToFiles", false, "If set, the JSON, SVG and rendered full SVG icon is written to files in the working directory.")
	AggressiveQuery = flag.Bool("aggressive", false, "If set, will connect to panels, query various info and disconnect.")
	WebServerPort = flag.Int("wsport", 8051, "Web server port")
	dontOpenBrowser := flag.Bool("dontOpenBrowser", false, "If set, a web browser won't open automatically")
	Dark = flag.Bool("dark", false, "If set, will render web UI in dark mode")
	RecordTriggers = flag.String("recordTriggers", "", "If set, will record triggers to the filename given as value")
	guiMode := flag.Bool("gui", false, "Run the application in GUI mode")

	flag.Parse()
	arguments := flag.Args()

	// Fyne setup:
	launchFyneGUI := *guiMode || checkIfPackaged()
	var gui = &FyneGui{}

	if launchFyneGUI {
		*WebServerPort = int(gui.Create(uint32(*WebServerPort), appName, appFriendlyName)) // Start the GUI
		log.Printf("Started at time %s\n", appLaunchTime.Format(time.RFC3339))             // Log the launch time
	}

	// Start webserver:
	if *WebServerPort > 0 {
		log.Infof("Starting webserver on localhost:%d\n", *WebServerPort)
		setupRoutes()
		go http.ListenAndServe(fmt.Sprintf(":%d", *WebServerPort), nil)

		if !(*dontOpenBrowser) {
			log.Infof("Opening Web Browser")
			go func() {
				time.Sleep(time.Millisecond * 500)
				openBrowser(fmt.Sprintf("http://localhost:%d", *WebServerPort))
			}()
		} else {
			log.Infof("Automatic opening of Web Browser disabled. Enable in Preferences.")
		}
	}

	wsClients = threadSafeSlice{}

	// Set up server:
	incoming = make(chan []*rwp.InboundMessage, 10)
	outgoing = make(chan []*rwp.OutboundMessage, 50)
	shadowPanelIncoming = make(chan []*rwp.InboundMessage, 10)

	demoHWCids.Store([]uint32{})

	go runZeroConfSearch()

	// Load default panel up, if set:
	if len(arguments) > 0 {
		switchToPanel(string(arguments[0]))
	}

	if len(arguments) > 1 {
		fmt.Println("Connection to shadow panel: ", string(arguments[1]))
		connectToShadowPanel(string(arguments[1]), shadowPanelIncoming)
	}

	// Wait forever:
	if launchFyneGUI {
		go func() {
			time.Sleep(time.Millisecond * 200) // Waiting for web server to start.

			// These messages gets printed out in reverse order in the Fyne UI:
			log.Println("**************************************************************")
			log.Println("*      Keep this window open in the background. Close it to quit.")
			log.Println("*      The application UI has opened in your web browser.")
			log.Println("**************************************************************")
		}()
		gui.ShowAndRun()
	} else {

		// Create an instance of the app structure
		app := NewApp()

		// Create application with options
		err := wails.Run(&options.App{
			Title:  "MyApp",
			Width:  1024,
			Height: 768,
			AssetServer: &assetserver.Options{
				Assets: assets,
			},
			BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
			OnStartup:        app.startup,
			Bind: []interface{}{
				app,
			},
		})

		if err != nil {
			println("Error:", err.Error())
		}

		for {
			select {}
		}
	}
}
