# raw-panel-explorer
A discovery and inspection application for SKAARHOJ Raw Panels

You find releases under the releases section on this repository

[CLICK HERE for Release downloads](https://github.com/SKAARHOJ/raw-panel-explorer/releases)

Here are some links to instructions for using Raw Panel Explorer on [Mac](https://wiki.skaarhoj.com/books/applications/page/running-cli-applications-mac) and [Windows](https://wiki.skaarhoj.com/books/applications/page/running-cli-applications-windows). These are intended for customers running the Command Line Interface (CLI) version.

There are also Mac and Windows applications that does not open a command line window. They are based on Fyne or Wails.


## Todo:
- Does go-routine leak in zeroconf monitor? Just like it did with the media players...?
- With shadow panels, compare typeDefs...
- Will crash, if a display is reported with 0x0 dimensions (which would be invalid anyway of course...)

## Exotic:
- "Bug": If a panel "changes identity" in flight, sometimes old typology information seems to be used when generating test graphics - despite the topology table showing correctly. This is probably a fairly exotic issue and seems fixed by a re-start, but why does it happen? Maybe only when a "connection" is taken over, like when connected to the raw-panel-dummies which suddenly change panel on the same port.

## Checkin out and running
Run with 

```bash
go run .
```

If you have issues with that, it may be because of the embedded filesystem from frontend/dist folder - in that case, create the folder and put some empty file inside of it.

## Build Commands for Fyne GUI
Fyne is a way to make a Go-lang based GUI application on Mac (or Windows). As a developer, if you want to package the Raw Panel Explorer as such, please install Fyne and use this command to package it ("darwin" = for Mac. Use "windows" for "Windows")

```bash
fyne package -os darwin
```

After doing so, Raw Panel Explorer (F).app is created in the root and you should edit the Raw Panel Explorer (F).app/Info.plist file by inserting this:

	<key>NSHumanReadableCopyright</key>
	<string>Copyright © 2024 Kasper Skårhøj</string>

Fyne will open a small support window which includes a log-window and a button to open a web browser with the UI. Closing the support window closes the application entirely.

For signing (will end up in zip file in binaries/ folder)
```bash
./buildAndSign_fyne.sh         
```

## Build Commands for Wails GUI
Wails is another way to package the Go-lang application as a native app. The way we use Wails is to simply wrap the webserver UI in an iframe opened in a webkit window. This is a super minimalistic approach to using Wails.

For development (continuously compiling when you save files), run this:

```bash
wails dev -tags runningwails  
```

For building, run this:

```bash
wails build -tags runningwails
```

For signing (will end up in zip file in binaries/ folder)
```bash
./buildAndSign_wails.sh         
```

Sometimes there are interesting issues with running wails which can often be fixed by deleting one or more of the directories inside of frontend/, specifically node_modules/, dist/ and wailsjs/ which seems all auto generated.

## Binaries folder:
The binaries named "PanelExplorer*" are unsigned command line versions
The binaries named "Raw Panel Explorer" are GUI applications for Mac (signed) and Windows (not signed). (F) means Fyne version for Mac (less used, legacy)
