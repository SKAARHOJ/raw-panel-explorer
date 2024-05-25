# raw-panel-explorer
A discovery and inspection application for SKAARHOJ Raw Panels

You find releases under the releases section on this repository

[CLICK HERE for Release downloads](https://github.com/SKAARHOJ/raw-panel-explorer/releases)

Here are some links to instructions for using Raw Panel Explorer on [Mac](https://wiki.skaarhoj.com/books/applications/page/running-cli-applications-mac) and [Windows](https://wiki.skaarhoj.com/books/applications/page/running-cli-applications-windows)


## Todo:
- Go-routine probably leaks in zeroconf monitor? Just like it did with the media players...?
- With shadow panels, compare typeDefs...
- Will crash, if a display is reported with 0x0 dimensions (which would be invalid anyway of course...)

## Exotic:
- "Bug": If a panel "changes identity" in flight, sometimes old typology information seems to be used when generating test graphics - despite the topology table showing correctly. This is probably a fairly exotic issue and seems fixed by a re-start, but why does it happen? Maybe only when a "connection" is taken over, like when connected to the raw-panel-dummies which suddenly change panel on the same port.