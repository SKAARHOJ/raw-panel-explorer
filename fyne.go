//go:build !(linux && arm64)

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	log "github.com/s00500/env_logger"
	"github.com/sirupsen/logrus"
)

type FyneGui struct {
	mainWindow fyne.Window
	fyneApp    fyne.App
}

func (gui *FyneGui) Create(WebServerPort uint32, appName, appFriendlyName string) uint32 {
	gui.mainWindow, gui.fyneApp, WebServerPort = createFyneWindow(WebServerPort, appName, appFriendlyName)

	return WebServerPort
}

func (gui *FyneGui) ShowAndRun() {
	gui.mainWindow.ShowAndRun()
}

// checkIfPackaged detects if the application is running in a packaged context.
func checkIfPackaged() bool {
	executablePath := os.Args[0]

	switch runtime.GOOS {
	case "darwin": // macOS
		// Packaged apps on macOS are usually in `.app` bundles
		return strings.Contains(executablePath, ".app")
	case "linux":
		// On Linux, you might check if the executable is in a standard installation path
		// or has a certain directory structure (e.g., /usr/bin or /opt/app).
		return strings.HasPrefix(executablePath, "/usr/") || strings.HasPrefix(executablePath, "/opt/")
	case "windows":
		// On Windows, check for `.exe` or specific installation paths
		// (e.g., Program Files or similar directory structure).
		// return strings.HasSuffix(executablePath, ".exe") &&
		// 	(strings.Contains(executablePath, "Program Files") || strings.Contains(executablePath, "AppData"))

		// Andreas: On windows since we dont "install" the app, we only need to check for .exe
		// Otherwise the UI will never really be shown, as in most cases it's probable gonna be run from the downloads or desktop folder
		return strings.HasSuffix(executablePath, ".exe")
	default:
		// Fallback for other platforms
		return false
	}
}

// TextWriter implements io.Writer and writes output to a Fyne text field
type TextWriter struct {
	textField *widget.Entry
	mu        sync.Mutex // Protects concurrent writes
}

// Write implements the io.Writer interface for TextWriter
func (tw *TextWriter) Write(p []byte) (n int, err error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// Append the new text to the text field
	tw.textField.SetText(parseLogLine(string(p)) + tw.textField.Text)
	return len(p), nil
}

func redirectOutput(writer io.Writer) {
	// Create pipes for stdout and stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	// Redirect stdout and stderr to pipes
	os.Stdout = wOut
	os.Stderr = wErr

	// Use goroutines to read from the pipes and write to the custom writer
	go func() {
		_, _ = io.Copy(writer, rOut)
	}()
	go func() {
		_, _ = io.Copy(writer, rErr)
	}()
}

func createFyneWindow(webserverPort uint32, appName, appFriendlyName string) (fyne.Window, fyne.App, uint32) {

	var appWidth = 600

	// Launch the Fyne GUI
	log.Println("Launching GUI mode and diverting logs to the GUI window...")

	// Create a new application
	fyneApp := app.NewWithID("com.skaarhoj." + appName)
	fyneApp.Settings().SetTheme(&customTheme{})

	mainWindow := fyneApp.NewWindow(appFriendlyName)
	mainWindow.Resize(fyne.NewSize(float32(appWidth), 400))

	wsportInt, err := strconv.Atoi(fyneApp.Preferences().StringWithFallback("wsport", fmt.Sprintf("%d", webserverPort)))
	if err == nil {
		webserverPort = uint32(wsportInt)
	}

	// Create a header for the log field with white and bold text
	logHeader := widget.NewLabelWithStyle("Log Output:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Create a multiline text entry
	textEntry := widget.NewMultiLineEntry()
	textEntry.SetPlaceHolder("Logs will appear here...")

	// Redirect stdout and stderr to the text field
	textWriter := &TextWriter{textField: textEntry}
	redirectOutput(textWriter)

	// Redirect the custom logger's output
	logger := logrus.New()
	logger.SetOutput(textWriter)
	logger.Formatter = &logrus.TextFormatter{}
	log.ConfigureAllLoggers(logger, "")

	// Create a button for opening the browser
	url := "http://localhost:" + fmt.Sprint(webserverPort)
	button := widget.NewButton(fmt.Sprintf(" Open Application  -  %s ", url), func() {
		log.Println(fmt.Sprintf("Opening Browser at %s...", url))
		openBrowser(url)
	})

	// Opening web browser:
	go func() {
		time.Sleep(0 * time.Millisecond)
		//log.Println("Opening Web Application...")
		openBrowser(url)
	}()

	// Load the logo
	logo := loadLogo(appWidth)

	// Web Socket Port entry
	WSPortEntry := widget.NewEntry()
	WSPortEntry.SetPlaceHolder(fmt.Sprintf("%d", webserverPort))
	WSPortEntry.SetText(fyneApp.Preferences().StringWithFallback("wsport", ""))

	saveButton := widget.NewButton("Save", func() {
		// Validate Webserver Port Entry
		portText := WSPortEntry.Text
		if portText == "" || isInteger(portText) {
			// Save preferences
			fyneApp.Preferences().SetString("wsport", portText)

			// Show success dialog
			dialog.ShowInformation("Success", "Preferences saved successfully.\nRestart application.", mainWindow)
		} else {
			// Clear the text field and show an error dialog if invalid
			WSPortEntry.SetText("")
			dialog.ShowError(fmt.Errorf("Web Server Port must be an integer"), mainWindow)
		}
	})

	ButtonsContainer := container.NewHBox(
		button,
		layout.NewSpacer(),
		widget.NewLabelWithStyle("Port:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(WSPortEntry, spacer(50, 0)),
		container.NewVBox(saveButton, spacer(0, 0)),
	)

	// Create a text input field and "Connect to Panel" button
	ipPortEntry := widget.NewEntry()
	ipPortEntry.SetPlaceHolder("IP:port")

	connectButton := widget.NewButton("Connect to Panel", func() {
		input := strings.TrimSpace(ipPortEntry.Text)
		ip, port, err := parseIPAndPort(input)
		if err != nil {
			log.Println("Error: Invalid IP and port:", err)
			dialog.ShowError(err, mainWindow)
			return
		}
		if port <= 0 {
			port = 9923
		}
		connectToPanelViaGUI(ip, port)
		log.Printf("Connecting to panel at %s:%d", ip, port)
	})

	// Layout for the input field and button
	connectContainer := container.NewGridWithColumns(2,
		ipPortEntry,   // Input field spans remaining space
		connectButton, // Button stays minimal
	)

	// Create the main layout
	appLayout := container.NewBorder(
		container.NewVBox(logo, spacer(0, 10), ButtonsContainer, spacer(0, 10), connectContainer, logHeader), // Top widgets
		nil,       // Bottom
		nil,       // Left
		nil,       // Right
		textEntry, // Center widget expands to fill remaining space
	)

	mainWindow.SetContent(appLayout)
	mainWindow.Show()

	return mainWindow, fyneApp, webserverPort
}

// Helper function to parse IP and port from the input
func parseIPAndPort(input string) (string, int, error) {
	parts := strings.Split(input, ":")
	if len(parts) == 0 || len(parts) > 2 {
		return "", 0, fmt.Errorf("invalid input format, expected IP:port")
	}

	ip := parts[0]
	if !isValidIPAddress(ip) {
		return "", 0, fmt.Errorf("invalid IP address")
	}

	port := 9923 // Default port
	if len(parts) == 2 && parts[1] != "" {
		parsedPort, err := strconv.Atoi(parts[1])
		if err != nil || parsedPort <= 0 {
			return "", 0, fmt.Errorf("invalid port number")
		}
		port = parsedPort
	}

	return ip, port, nil
}

// Helper function to validate IP address
func isValidIPAddress(ip string) bool {
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return false
	}
	for _, octet := range octets {
		num, err := strconv.Atoi(octet)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}
	return true
}

// Function to connect to the panel (dummy implementation)
func connectToPanelViaGUI(ip string, port int) {
	log.Printf("Connecting to panel at %s:%d...", ip, port)
	switchToPanel(fmt.Sprintf("%s:%d", ip, port))
}

func parseLogLine(logLine string) string {
	// Extract fields from the log line
	fields := make(map[string]string)
	start := 0
	for start < len(logLine) {
		// Find the next key=
		eqIndex := strings.Index(logLine[start:], "=")
		if eqIndex == -1 {
			break
		}
		eqIndex += start

		// Find the key
		key := strings.TrimSpace(logLine[start:eqIndex])

		// Find the value
		valueStart := eqIndex + 1
		var value string
		if logLine[valueStart] == '"' { // Handle quoted values
			valueEnd := strings.Index(logLine[valueStart+1:], "\"") + valueStart + 1
			if valueEnd == valueStart { // No closing quote
				break
			}
			rawValue := logLine[valueStart : valueEnd+1] // Include the quotes
			unquotedValue, err := strconv.Unquote(rawValue)
			if err != nil {
				return fmt.Sprintf("Error unquoting value: %v", err)
			}
			value = unquotedValue
			start = valueEnd + 1
		} else { // Handle unquoted values
			valueEnd := strings.IndexAny(logLine[valueStart:], " \n")
			if valueEnd == -1 {
				value = logLine[valueStart:]
				start = len(logLine)
			} else {
				value = logLine[valueStart : valueStart+valueEnd]
				start = valueStart + valueEnd + 1
			}
		}

		// Store the key-value pair
		fields[key] = value
	}

	// Parse the timestamp
	timestamp, err := time.Parse(time.RFC3339, fields["time"])
	if err != nil {
		return fmt.Sprintf("Error parsing timestamp: %v", err)
	}

	// Calculate the duration since the application launched
	duration := timestamp.Sub(appLaunchTime).Truncate(time.Second)
	durationStr := fmt.Sprintf("%02d:%02d:%02d", int(duration.Hours()), int(duration.Minutes())%60, int(duration.Seconds())%60)

	// Format the log level
	logLevel := strings.ToUpper(fields["level"])
	logLevelFormatted := fmt.Sprintf("[%s]", logLevel)

	// Clean up the message
	message := strings.TrimSpace(fields["msg"]) + "\n"

	// Construct the formatted log string
	return fmt.Sprintf("%s %s %s", durationStr, logLevelFormatted, message)
}

func loadLogo(appWidth int) *canvas.Image {
	img, _, err := image.Decode(bytes.NewReader(ReadResourceFile("resources/fynelogo.png")))
	if err != nil {
		log.Fatalf("Failed to decode embedded image: %v", err)
	}

	logo := canvas.NewImageFromImage(img)
	logo.FillMode = canvas.ImageFillContain
	ratio := 1675 / appWidth
	logo.SetMinSize(fyne.NewSize(float32(1312/ratio), float32(94/ratio)))

	return logo
}

func spacer(width, height int) *canvas.Rectangle {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(float32(width), float32(height)))
	return spacer
}

// Helper Function to Check if a String is an Integer
func isInteger(input string) bool {
	_, err := strconv.Atoi(input)
	return err == nil
}

type customTheme struct{}

func (customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return color.NRGBA{R: 34, G: 34, B: 34, A: 255} // Custom background color
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (customTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
