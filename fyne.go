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

func createFyneWindow(webserverPort uint32, dontOpenBrowser bool) (fyne.Window, fyne.App) {
	// Launch the Fyne GUI
	log.Println("Launching GUI mode and diverting logs to the GUI window...")

	// Create a new application
	myApp := app.NewWithID("com.skaarhoj.raw-panel-explorer")
	myApp.Settings().SetTheme(&customTheme{})

	dontOpenBrowser = !myApp.Preferences().BoolWithFallback("openBrowserOnStartup", !dontOpenBrowser)

	mainWindow := myApp.NewWindow("Raw Panel Explorer")
	mainWindow.Resize(fyne.NewSize(float32(appWidth), 400))

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
	buttonContainer := container.NewHBox()
	if dontOpenBrowser {
		button := widget.NewButton("Open Browser", func() {
			openBrowser("http://localhost:" + fmt.Sprint(webserverPort))
			log.Println("Opening browser...")
		})

		// Wrap the button in a centered horizontal box
		buttonContainer = container.NewHBox(
			button,
		)
	}

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
		connectToPanelViaGUI(ip, port)
		log.Printf("Connecting to panel at %s:%d", ip, port)
	})

	// Layout for the input field and button
	connectContainer := container.NewGridWithColumns(2,
		ipPortEntry,   // Input field spans remaining space
		connectButton, // Button stays minimal
	)

	// Load the logo
	logo := loadLogo()

	// Create the main layout
	appLayout := container.NewBorder(
		container.NewVBox(logo, buttonContainer, connectContainer, logHeader), // Top widgets
		nil,       // Bottom
		nil,       // Left
		nil,       // Right
		textEntry, // Center widget expands to fill remaining space
	)

	mainWindow.SetContent(appLayout)
	mainWindow.SetMainMenu(createAppMenu(myApp, mainWindow))
	mainWindow.Show()

	return mainWindow, myApp
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
		return strings.HasSuffix(executablePath, ".exe") &&
			(strings.Contains(executablePath, "Program Files") || strings.Contains(executablePath, "AppData"))
	default:
		// Fallback for other platforms
		return false
	}
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

func loadLogo() *canvas.Image {
	img, _, err := image.Decode(bytes.NewReader(ReadResourceFile("resources/logo.png")))
	if err != nil {
		log.Fatalf("Failed to decode embedded image: %v", err)
	}

	logo := canvas.NewImageFromImage(img)
	logo.FillMode = canvas.ImageFillContain
	ratio := 1675 / appWidth
	logo.SetMinSize(fyne.NewSize(float32(1312/ratio), float32(94/ratio)))

	return logo
}

func createAppMenu(myApp fyne.App, mainWindow fyne.Window) *fyne.MainMenu {
	return fyne.NewMainMenu(
		fyne.NewMenu("",
			fyne.NewMenuItem("Preferences", func() {
				showPreferences(myApp, mainWindow)
			}),
		),
	)
}

func spaceAbove(height int) *canvas.Rectangle {
	spaceAbove := canvas.NewRectangle(color.Transparent)
	spaceAbove.SetMinSize(fyne.NewSize(0, 10))
	return spaceAbove
}

// Preferences Dialog Function
func showPreferences(myApp fyne.App, parent fyne.Window) {

	preferencesWindow := myApp.NewWindow("Preferences")
	preferencesWindow.Resize(fyne.NewSize(600, 400))

	// Folder ID Header with Help Icon
	WSPortHeader := container.NewHBox(
		widget.NewLabelWithStyle("Web Server Port:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
			showHelpDialog(preferencesWindow)
		}),
	)

	// Folder ID Entry
	WSPortEntry := widget.NewEntry()
	WSPortEntry.SetPlaceHolder("8051")
	WSPortEntry.SetText(myApp.Preferences().StringWithFallback("wsport", ""))

	// Checkbox for "Open Web Browser on Startup"
	openBrowserCheckbox := widget.NewCheck("Open Web Browser on Startup", nil)
	openBrowserCheckbox.SetChecked(myApp.Preferences().BoolWithFallback("openBrowserOnStartup", true))

	// Space Above Save Button
	spaceAbove := canvas.NewRectangle(color.Transparent)
	spaceAbove.SetMinSize(fyne.NewSize(0, 20)) // 20 pixels of space

	// Save Button
	saveButton := widget.NewButton("Save", func() {
		// Validate Webserver Port Entry
		portText := WSPortEntry.Text
		if portText == "" || isInteger(portText) {
			// Save preferences
			myApp.Preferences().SetString("wsport", portText)
			myApp.Preferences().SetBool("openBrowserOnStartup", openBrowserCheckbox.Checked)

			// Show success dialog
			dialog.ShowInformation("Success", "Preferences saved successfully.", preferencesWindow)
			preferencesWindow.Close()
		} else {
			// Clear the text field and show an error dialog if invalid
			WSPortEntry.SetText("")
			dialog.ShowError(fmt.Errorf("Web Server Port must be an integer"), preferencesWindow)
		}
	})

	// Preferences Layout
	preferencesWindow.SetContent(container.NewVBox(
		WSPortHeader,
		WSPortEntry,
		openBrowserCheckbox,
		spaceAbove,
		container.NewHBox(
			layout.NewSpacer(),
			saveButton,
		),
	))
	preferencesWindow.Show()
}

// Helper Function to Check if a String is an Integer
func isInteger(input string) bool {
	_, err := strconv.Atoi(input)
	return err == nil
}

// Shows the help dialog for Folder ID
func showHelpDialog(parent fyne.Window) {
	richText := widget.NewRichText(
		&widget.TextSegment{Text: "The main UI of this application runs in a web browser. In case the default port is already in use, you can specify an alternative port here.\n"},
		// &widget.TextSegment{
		// 	Text:  "567oeIUYeddWcho14g5bXULFJD7Db5vO6",
		// 	Style: widget.RichTextStyleStrong, // Bold
		// },
	)
	dialog.ShowCustom("What is the Web Server Port?", "Close", container.NewVBox(richText), parent)
}

// Updates the status text and color
func updateStatusText(statusText *canvas.Text, credentialsNotEmpty bool) {
	if credentialsNotEmpty {
		statusText.Text = "Credentials Uploaded."
		statusText.Color = color.RGBA{R: 0, G: 128, B: 0, A: 255} // Green
	} else {
		statusText.Text = "No credentials uploaded."
		statusText.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red
	}
	statusText.Refresh()
}
