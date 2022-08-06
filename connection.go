package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	su "github.com/SKAARHOJ/ibeam-lib-utils"
	helpers "github.com/SKAARHOJ/rawpanel-lib"
	rwp "github.com/SKAARHOJ/rawpanel-lib/ibeam_rawpanel"

	log "github.com/s00500/env_logger"

	topology "github.com/SKAARHOJ/rawpanel-lib/topology"

	"go.uber.org/atomic"
)

var lastState *wsToClient
var lastStateMu sync.Mutex

var incoming chan []*rwp.InboundMessage
var outgoing chan []*rwp.OutboundMessage

var panelConnectionCancel *context.CancelFunc
var waitForShutdown sync.WaitGroup

var TopologyData = &topology.Topology{}
var TotalUptimeGlobal uint32

var isConnected atomic.Bool

func getTopology(incoming chan []*rwp.InboundMessage, outgoing chan []*rwp.OutboundMessage, ctx context.Context) {

	var sendStateToClient atomic.Bool
	var HWCavailabilityMap = make(map[uint32]uint32)
	var ReceivedTopology bool
	var IsSleeping bool
	var SendTopMutex sync.Mutex

	topologyJSON := ""
	topologySVG := ""

	t := time.NewTicker(time.Millisecond * 500)

	for {
		select {
		case <-ctx.Done():
			log.Debugln("Context cancelled getTopology()")
			isConnected.Store(false)
			wsslice.Iter(func(w *wsclient) {
				w.msgToClient <- &wsToClient{
					DisconnectedSignal: true,
				}
			})
			t.Stop()
			return
		case outboundMessages := <-outgoing:
			// First, print the lines coming in as ASCII:
			lines := helpers.OutboundMessagesToRawPanelASCIIstrings(outboundMessages)
			for _, line := range lines {
				if *PanelToSystemMessages {
					fmt.Println(string("Panel -> System: " + strings.TrimSpace(string(line))))
				}
			}

			// Next, do some processing on it:
			SendTopMutex.Lock()
			for _, msg := range outboundMessages {

				if msg.PanelTopology != nil {
					if msg.PanelTopology.Json != "" {
						ReceivedTopology = true
						err := json.Unmarshal([]byte(msg.PanelTopology.Json), TopologyData)
						if err != nil {
							log.Println("Topology JSON parsing Error: ", err)
						} else {
							//fmt.Println("Received Topology JSON")
							topologyJSON = msg.PanelTopology.Json
							//log.Println(log.Indent(topologyData))
						}
					}
					if msg.PanelTopology.Svgbase != "" {
						ReceivedTopology = true
						topologySVG = msg.PanelTopology.Svgbase
						//	fmt.Println("Received Topology SVG")
					}
				}

				if msg.PanelInfo != nil {
					if !isConnected.Load() {
						wsslice.Iter(func(w *wsclient) {
							w.msgToClient <- &wsToClient{
								ConnectedSignal: true,
							}
						})
					}
					isConnected.Store(true)

					lastStateMu.Lock()
					if msg.PanelInfo.Name != "" {
						lastState.Title = msg.PanelInfo.Name
					}
					if msg.PanelInfo.Model != "" {
						lastState.Model = msg.PanelInfo.Model
					}
					if msg.PanelInfo.Serial != "" {
						lastState.Serial = msg.PanelInfo.Serial
					}
					if msg.PanelInfo.SoftwareVersion != "" {
						lastState.SoftwareVersion = msg.PanelInfo.SoftwareVersion
					}
					if msg.PanelInfo.Platform != "" {
						lastState.Platform = msg.PanelInfo.Platform
					}
					if msg.PanelInfo.BluePillReady {
						lastState.BluePillReady = "Yes"
					}
					if msg.PanelInfo.MaxClients != 0 {
						lastState.MaxClients = msg.PanelInfo.MaxClients
					}
					if len(msg.PanelInfo.LockedToIPs) != 0 {
						lastState.LockedToIPs = strings.Join(msg.PanelInfo.LockedToIPs, ";")
					}

					lastState.Time = getTimeString()
					lastStateMu.Unlock()
				}

				if msg.FlowMessage == 1 { // Ping:
					incoming <- []*rwp.InboundMessage{
						&rwp.InboundMessage{
							FlowMessage: 2,
						},
					}
				}

				if msg.FlowMessage == 5 { // RDY
					wsslice.Iter(func(w *wsclient) {
						w.msgToClient <- &wsToClient{
							RDYBSY: "<span style='color: red;'>BSY</span>",
						}
					})
				}

				if msg.FlowMessage == 5 { // BSY
					wsslice.Iter(func(w *wsclient) {
						w.msgToClient <- &wsToClient{
							RDYBSY: "<span style='color: green;'>RDY</span>",
						}
					})
				}

				if msg.SleepState != nil { // Sleeping flag
					IsSleeping = msg.SleepState.IsSleeping
					wsslice.Iter(func(w *wsclient) {
						w.msgToClient <- &wsToClient{
							Sleeping: su.Qstr(msg.SleepState.IsSleeping, "<span style='color: orange;'>Sleeping</span>", "<span style='color: green;'>Awake</span>"),
						}
					})
				}

				if msg.Connections != nil {
					lastStateMu.Lock()
					lastState.Connections = strings.Join(msg.Connections.Connection, " ") + " "
					lastStateMu.Unlock()
					sendStateToClient.Store(true)
				}
				if msg.RunTimeStats != nil {
					lastStateMu.Lock()
					if msg.RunTimeStats.BootsCount > 0 {
						lastState.BootsCount = msg.RunTimeStats.BootsCount
					}
					if msg.RunTimeStats.TotalUptime > 0 {
						TotalUptimeGlobal = msg.RunTimeStats.TotalUptime // Because we need the value below and these may not come in the same message (they DONT on ASCII version of RWP protocol...)
						lastState.TotalUptime = fmt.Sprintf("%dd %dh", msg.RunTimeStats.TotalUptime/60/24, (msg.RunTimeStats.TotalUptime/60)%24)
					}
					if msg.RunTimeStats.SessionUptime > 0 {
						lastState.SessionUptime = fmt.Sprintf("%dh %dm", msg.RunTimeStats.SessionUptime/60, msg.RunTimeStats.SessionUptime%60)
					}
					if msg.RunTimeStats.ScreenSaveOnTime > 0 {
						pct := -1
						if TotalUptimeGlobal > 0 {
							pct = 100 * int(msg.RunTimeStats.ScreenSaveOnTime) / int(TotalUptimeGlobal)
						}
						lastState.ScreenSaveOnTime = fmt.Sprintf("%dd %dh (%d%%)", msg.RunTimeStats.ScreenSaveOnTime/60/24, (msg.RunTimeStats.ScreenSaveOnTime/60)%24, pct)
					}
					lastStateMu.Unlock()
					sendStateToClient.Store(true)
				}

				// Picking up availability information (map command)
				if msg.HWCavailability != nil && !IsSleeping { // Only update the map internally if the panel is not asleep. Luckily the sleep indication will arrive before the updated map, so we can use this to prevent the map from being updated.
					//log.Println(log.Indent(msg.HWCavailability))
					for HWCid, MappedTo := range msg.HWCavailability {
						sendStateToClient.Store(true)
						HWCavailabilityMap[HWCid] = MappedTo
					}
				}

				if msg.SysStat != nil {
					wsslice.Iter(func(w *wsclient) {
						w.msgToClient <- &wsToClient{
							CPUState: fmt.Sprintf("%.1fC, %d%%, %dMHz", msg.SysStat.CPUTemp, msg.SysStat.CPUUsage, msg.SysStat.CPUFreqCurrent/1000),
						}
					})
				}

				if msg.Events != nil {
					for _, Event := range msg.Events {
						eventMessage := &wsToClient{
							PanelEvent: Event,
							Time:       getTimeString(),
						}
						wsslice.Iter(func(w *wsclient) { w.msgToClient <- eventMessage })

						eventPlot(Event)
					}
				}
			}
			SendTopMutex.Unlock()
		case <-t.C: // Send topology based on a timer so that we don't trigger it on every received map command for example. Rather, state for map and topology will be pooled together and forwarded every half second.
			SendTopMutex.Lock()
			if (ReceivedTopology || sendStateToClient.Load()) && (topologyJSON != "" && topologySVG != "") {
				//fmt.Println("ReceivedTopology, sendStateToClient", ReceivedTopology, sendStateToClient)
				ReceivedTopology = false
				sendStateToClient.Store(false)
				//log.Println(log.Indent(HWCavailabilityMap))

				svgIcon := topology.GenerateCompositeSVG(topologyJSON, topologySVG, HWCavailabilityMap)

				regex := regexp.MustCompile(`id="HWc([0-9]+)"`)
				svgIcon = regex.ReplaceAllString(svgIcon, fmt.Sprintf("id=\"SVG_HWc$1\" onclick=\"clickHWC(evt,$1)\"onclick=\"clickHWC(event,$1)\" onmouseenter=\"this.setAttribute('stroke-width','10');this.setAttribute('stroke','red');\" onmouseout=\"this.setAttribute('stroke-width','2');this.setAttribute('stroke','#000');\""))

				topOverviewTable := GenerateTopologyOverviewTable(topologyJSON, HWCavailabilityMap)
				topOverviewTable = regex.ReplaceAllString(topOverviewTable, fmt.Sprintf("id=\"Row_HWc$1\""))

				//fmt.Println(topOverviewTable)

				// Create a JSON object to marshal in a pretty format
				shadowPanelTopologyData := &topology.Topology{}
				json.Unmarshal([]byte(topologyJSON), shadowPanelTopologyData)
				s, _ := json.MarshalIndent(shadowPanelTopologyData, "", "  ")
				topJson := string(s)

				// Horrible, but functional processing of the JSON to insert some HTML to be able to highlight the HWCs
				regex = regexp.MustCompile(`"id": ([0-9]+),`)
				topJsonPartsBegin := strings.Split(topJson, "\n    {\n")
				for i := range topJsonPartsBegin {
					topJsonParts := strings.Split(topJsonPartsBegin[i], "\n    }")

					matches := regex.FindStringSubmatch(topJsonParts[0])
					if matches != nil {
						topJsonParts[0] = fmt.Sprintf(`<span id="Top_HWc%s" onclick="clickHWC(event,%s)">`, matches[1], matches[1]) + topJsonParts[0] + `</span>`
					}
					topJsonPartsBegin[i] = strings.Join(topJsonParts, "\n    }")
				}
				topJson = strings.Join(topJsonPartsBegin, "\n    {\n")
				//fmt.Println(topJson)

				if *writeTopologiesToFiles {
					// Process it...
					f, _ := os.Create("_topologySVGIconFullRender.svg")
					f.WriteString(svgIcon)
					f.Sync()
					f.Close()

					f, _ = os.Create("_topology.svg")
					f.WriteString(topologySVG)
					f.Sync()
					f.Close()

					f, _ = os.Create("_topology.json")
					f.WriteString(string(s))
					f.Sync()
					f.Close()
				}

				lastStateMu.Lock()
				lastState.SvgIcon = svgIcon
				lastState.TopologyTable = topOverviewTable
				lastState.TopologyJSON = topJson
				lastState.Time = getTimeString()
				wsslice.Iter(func(w *wsclient) { w.msgToClient <- lastState })
				lastStateMu.Unlock()
			}
			SendTopMutex.Unlock()
		}
	}
}

func switchToPanel(panelIPAndPort string) {

	// Kill old connection
	if panelConnectionCancel != nil {
		log.Print("Disconnected based on switching panel, waiting for shutdown...")
		(*panelConnectionCancel)()
		waitForShutdown.Wait()
		log.Println("Shutdown done!")
	}

	// Set up new:
	ctx, cancel := context.WithCancel(context.Background())
	panelConnectionCancel = &cancel

	lastStateMu.Lock()
	lastState = &wsToClient{
		Title:         "-",
		Model:         "-",
		Serial:        "-",
		SvgIcon:       "-",
		TopologyTable: "-",
		Time:          time.Now().String(),
	}
	lastStateMu.Unlock()

	// On-connect function - asking for a bunch of things...:
	onconnect := func(errorMsg string, binary bool, c net.Conn) {
		log.Printf("Connected to %s\n", panelIPAndPort)

		// Send query for stuff we want to know...:
		incoming <- []*rwp.InboundMessage{
			&rwp.InboundMessage{
				Command: &rwp.Command{
					ActivatePanel:         true,
					SendPanelInfo:         true,
					SendPanelTopology:     true,
					ReportHWCavailability: true,
					GetConnections:        true,
					GetRunTimeStats:       true,
					PublishSystemStat: &rwp.PublishSystemStat{
						PeriodSec: 15,
					},
					SetHeartBeatTimer: &rwp.HeartBeatTimer{
						Value: 3000,
					},
				},
			},
		}
	}

	passOnIncoming := make(chan []*rwp.InboundMessage, 10)
	go func() {
		//a := 0
		poll := time.NewTicker(time.Millisecond * 60 * 1000)
		for {
			select {
			case <-ctx.Done():
				poll.Stop()
				return
			case incomingMessages := <-incoming:
				passOnIncoming <- incomingMessages
				if shadowPanelListening.Load() {
					shadowPanelIncoming <- incomingMessages
				}
			case <-poll.C:
				incoming <- []*rwp.InboundMessage{
					&rwp.InboundMessage{
						Command: &rwp.Command{
							GetConnections:  true,
							GetRunTimeStats: true,
						},
					},
				}
			}
		}
	}()

	//go connectToPanel(panelIPAndPort, incoming, outgoing, ctx)
	go helpers.ConnectToPanel(panelIPAndPort, passOnIncoming, outgoing, ctx, &waitForShutdown, onconnect, nil, nil)

	go getTopology(incoming, outgoing, ctx)
}
