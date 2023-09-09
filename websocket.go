package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"sync"

	helpers "github.com/SKAARHOJ/rawpanel-lib"
	"github.com/SKAARHOJ/rawpanel-lib/ibeam_rawpanel"
	rwp "github.com/SKAARHOJ/rawpanel-lib/ibeam_rawpanel"
	rawpanelproc "github.com/SKAARHOJ/rawpanel-processors"
	"github.com/gorilla/websocket"
	log "github.com/s00500/env_logger"
	"google.golang.org/protobuf/proto"
)

type wsToClient struct {
	Title            string `json:",omitempty"`
	Model            string `json:",omitempty"`
	Serial           string `json:",omitempty"`
	SoftwareVersion  string `json:",omitempty"`
	Platform         string `json:",omitempty"`
	BluePillReady    string `json:",omitempty"`
	MaxClients       uint32 `json:",omitempty"`
	LockedToIPs      string `json:",omitempty"`
	Connections      string `json:",omitempty"`
	BootsCount       uint32 `json:",omitempty"`
	TotalUptime      string `json:",omitempty"`
	SessionUptime    string `json:",omitempty"`
	ScreenSaveOnTime string `json:",omitempty"`
	RDYBSY           string `json:",omitempty"`
	Sleeping         string `json:",omitempty"`
	CPUState         string `json:",omitempty"`
	EnvHealth        string `json:",omitempty"`

	SvgIcon       string `json:",omitempty"`
	TopologyTable string `json:",omitempty"`
	TopologyJSON  string `json:",omitempty"`
	Time          string `json:",omitempty"`
	ControlBlock  string `json:",omitempty"`

	PanelEvent *rwp.HWCEvent `json:",omitempty"`
	RWPState   *rwp.HWCState `json:",omitempty"`

	RWPASCIIToPanel    string `json:",omitempty"`
	RWPProtobufToPanel string `json:",omitempty"`
	RWPJSONToPanel     string `json:",omitempty"`
	StepDescription    string `json:",omitempty"`

	ConnectedSignal    bool `json:",omitempty"`
	DisconnectedSignal bool `json:",omitempty"`

	ZeroconfEntries []*ZeroconfEntry
}

type wsFromClient struct {
	RWPState             *rwp.HWCState `json:",omitempty"`
	RequestControlForHWC int           `json:",omitempty"`

	ConnectTo  string `json:",omitempty"`
	Disconnect bool   `json:",omitempty"`

	Command *rwp.Command `json:",omitempty"`

	FullThrottle bool `json:",omitempty"`

	DemoVUStart  bool     `json:",omitempty"`
	DemoStart    bool     `json:",omitempty"`
	DemoStop     bool     `json:",omitempty"`
	DemoHWCs     []uint32 `json:",omitempty"`
	DemoBackward bool     `json:",omitempty"`
	DemoForward  bool     `json:",omitempty"`

	Image_HWCIDs []int  `json:",omitempty"`
	ImageMode    string `json:",omitempty"`
	ImageData    []byte `json:",omitempty"`

	FullStateData []byte `json:",omitempty"`
}

type wsclient struct {
	msgToClient chan []byte
	quit        chan bool
}

type threadSafeSlice struct {
	sync.Mutex
	wsclients []*wsclient
}

type StateData struct {
	States     map[uint32]*ibeam_rawpanel.HWCState // Recorded state info
	Timestamps map[uint32]time.Time                // Time stamps of recorded state (used to make sure that overlapping image tiles are rendered in the order they came in.)
}
type stateTimeAndHWC struct {
	Timestamp time.Time
	HWCId     uint32
}

func (slice *threadSafeSlice) Push(w *wsclient) {
	wsClients.Lock()
	defer wsClients.Unlock()
	wsClients.wsclients = append(wsClients.wsclients, w)
}

func (slice *threadSafeSlice) Pull(w *wsclient) bool {
	wsClients.Lock()
	defer wsClients.Unlock()
	for i, wsclient := range wsClients.wsclients {
		if wsclient == w {
			wsClients.wsclients = append(wsClients.wsclients[:i], wsClients.wsclients[i+1:]...)
			//fmt.Println("Removed ", i)
			return true
		}
	}
	return false
}

func (slice *threadSafeSlice) Iter(routine func(*wsclient)) {
	wsClients.Lock()
	defer wsClients.Unlock()
	for _, wsclient := range wsClients.wsclients {
		routine(wsclient)
	}
}

var wsClients threadSafeSlice

func (w *wsclient) Start(ws *websocket.Conn) {
	w.msgToClient = make(chan []byte, 10) // some buffer size to avoid blocking
	go func() {
		for {
			select {
			case msg := <-w.msgToClient:
				err := ws.WriteMessage(websocket.TextMessage, msg)
				//_ = err // We need to remove old ws clients, they are piling up as the errors will show. But I couldn't immediately see how... (KS)
				if err != nil {
					go func() { // This is wrapped in a go-routine since otherwise iteration over things to send would result in a lock on the wsserver mutex inside Push/Pull/Iter. This seems to fix it...:
						//log.Println("Removing a Client")
						if !wsClients.Pull(w) {
							//log.Should(log.Wrap(err, "on writing to ws client. Tried to remove it, but nothing was removed..."))
						}
						//log.Println("Done Removing a Client")
					}()
				}
			case <-w.quit:
				return
			}
		}
	}()
}

func BroadcastMessage(msg *wsToClient) {
	// This function makes the marshal syncronous and keeps it on the same goroutine, also only happens once now
	msgAsString, err := json.Marshal(msg)
	log.Should(log.Wrap(err, "on marshalling message to client"))
	wsClients.Iter(func(w *wsclient) { w.msgToClient <- msgAsString })
}

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func homePage(w http.ResponseWriter, r *http.Request) {
	darkCSS := ""
	if *Dark {
		darkCSS = string(ReadResourceFile("resources/dark.css"))
	}
	fmt.Fprint(w, strings.ReplaceAll(string(ReadResourceFile("resources/index.html")), "/*INSERTDARK*/", darkCSS))
}
func panelPage(w http.ResponseWriter, r *http.Request) {
	darkCSS := ""
	if *Dark {
		darkCSS = string(ReadResourceFile("resources/dark.css"))
	}
	fmt.Fprint(w, strings.ReplaceAll(string(ReadResourceFile("resources/panel.html")), "/*INSERTDARK*/", darkCSS))
}
func logoheaderPage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, string(ReadResourceFile("resources/logoheader.png")))
}
func kasperwasherePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, string(ReadResourceFile("resources/kasperwashere.png")))
}
func colorPickerJS(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, string(ReadResourceFile("resources/vanilla-picker.js")))
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	ww := &wsclient{}
	ww.Start(ws)
	wsClients.Push(ww)

	// listen indefinitely for new messages coming
	// through on our WebSocket connection
	reader(ws)
	ww.quit <- true
	fmt.Println("Exit")
}

// define a reader which will listen for
// new messages being sent to our WebSocket
// endpoint
func reader(conn *websocket.Conn) {
	for {
		// read in a message
		_, p, err := conn.ReadMessage()
		if err != nil {
			//log.Println(err)
			return
		}
		// print out that message for clarity
		switch string(p) {
		case "SendAll":
			lastStateMu.Lock()
			BroadcastMessage(lastState)
			lastStateMu.Unlock()
		case "SendIndex":
			ZEntries.DeepRLock()
			BroadcastMessage(&wsToClient{
				ZeroconfEntries: ZEntries.entries,
				Time:            getTimeString(),
			})
			ZEntries.DeepRUnlock()

			BroadcastMessage(&wsToClient{
				ConnectedSignal: isConnected.Load(),
			})

		default:
			wsFromClient := &wsFromClient{}
			err := json.Unmarshal(p, wsFromClient)
			log.Should(err)
			//log.Println("Received from websocket: ", log.Indent(wsFromClient))

			if wsFromClient.RequestControlForHWC > 0 {
				BroadcastMessage(&wsToClient{
					RWPState: &rwp.HWCState{},
				})
			}

			if wsFromClient.ConnectTo != "" {
				switchToPanel(wsFromClient.ConnectTo)
			}

			if wsFromClient.Disconnect {
				if panelConnectionCancel != nil {
					log.Print("Disconnected based on ws disconnect message, waiting for shutdown...")
					(*panelConnectionCancel)()
					waitForShutdown.Wait()
					log.Println("Shutdown done!")
				}
			}

			if wsFromClient.Command != nil {
				stopDemos()
				//log.Println(log.Indent(wsFromClient.Command))
				incomingMessages := []*rwp.InboundMessage{
					&rwp.InboundMessage{
						Command: wsFromClient.Command,
					},
				}
				incoming <- incomingMessages
			}

			if wsFromClient.FullThrottle {
				stopDemos()
				//fmt.Println("Turning Everything On:")

				HWCids := []uint32{}
				for _, HWcDef := range TopologyData.HWc {
					HWCids = append(HWCids, HWcDef.Id)
				}
				incomingMessages := []*rwp.InboundMessage{
					&rwp.InboundMessage{
						Command: &rwp.Command{
							PanelBrightness: &rwp.Brightness{
								OLEDs: 8,
								LEDs:  8,
							},
						},
						States: []*rwp.HWCState{
							&rwp.HWCState{
								HWCIDs: HWCids,
								HWCMode: &rwp.HWCMode{
									State: 4,
								},
								HWCExtended: &rwp.HWCExtended{},
								HWCColor: &rwp.HWCColor{
									ColorIndex: &rwp.ColorIndex{
										Index: 0,
									},
								},
								HWCText: &rwp.HWCText{
									Formatting: 7,
									Inverted:   true,
								},
							},
						},
					},
				}
				incoming <- incomingMessages
			}

			if wsFromClient.FullStateData != nil {
				stopDemos()

				theStateData := &StateData{}
				err := json.Unmarshal(wsFromClient.FullStateData, theStateData)
				log.Should(err)
				log.Println(theStateData)

				// First, we will create a little slice where we note all last update times for HWC states, then sort it and then traverse it in the order. This is necessary to make sure overlapping tiles get written in the right order.
				loopOrder := make([]*stateTimeAndHWC, 0, len(theStateData.States))

				for hwcId := range theStateData.States {
					timeStamp := time.Now()
					if theStateData.Timestamps != nil {
						if _, exists := theStateData.Timestamps[hwcId]; exists { // If there are timestamps we will send the states in a specific order...
							timeStamp = theStateData.Timestamps[hwcId]
							fmt.Println(hwcId, timeStamp)
						}
					}
					loopOrder = append(loopOrder, &stateTimeAndHWC{Timestamp: timeStamp, HWCId: hwcId})
				}
				sort.Slice(loopOrder, func(i, j int) bool {
					return loopOrder[i].Timestamp.Before(loopOrder[j].Timestamp)
				})
				log.Println(log.Indent(loopOrder))

				for _, item := range loopOrder {
					theStateData.States[item.HWCId].HWCIDs = []uint32{item.HWCId}
					incomingMessages := []*rwp.InboundMessage{
						{
							States: []*rwp.HWCState{
								theStateData.States[item.HWCId],
							},
						},
					}
					incoming <- incomingMessages
				}
			}

			if wsFromClient.DemoStart {
				HWCids := wsFromClient.DemoHWCs
				if len(HWCids) == 0 {
					for _, HWcDef := range TopologyData.HWc {
						HWCids = append(HWCids, HWcDef.Id)
					}
				}
				startDemo(HWCids)
			}
			if wsFromClient.DemoVUStart {
				HWCids := wsFromClient.DemoHWCs
				if len(HWCids) == 0 {
					for _, HWcDef := range TopologyData.HWc {
						HWCids = append(HWCids, HWcDef.Id)
					}
				}
				startVUDemo(HWCids)
			}
			if wsFromClient.DemoStop {
				stopDemos()
			}
			if wsFromClient.DemoBackward {
				stepBackward()
			}
			if wsFromClient.DemoForward {
				stepForward()
			}

			if wsFromClient.RWPState != nil {
				stopDemos()
				//log.Println("Received State Change from Client: ", log.Indent(wsFromClient.RWPState))

				/*
					// If empty HWCMode structs are removed, we won't see triggers like "Off".
					if proto.Equal(wsFromClient.RWPState.HWCMode, &rwp.HWCMode{}) {
						wsFromClient.RWPState.HWCMode = nil
					} */
				if proto.Equal(wsFromClient.RWPState.HWCColor, &rwp.HWCColor{}) {
					wsFromClient.RWPState.HWCColor = nil
				}
				/*if proto.Equal(wsFromClient.RWPState.HWCExtended, &rwp.HWCExtended{}) {
					wsFromClient.RWPState.HWCExtended = &rwp.HWCExtended{}
				}*/
				if proto.Equal(wsFromClient.RWPState.HWCText, &rwp.HWCText{}) {
					wsFromClient.RWPState.HWCText = nil
				}

				incomingMessages := []*rwp.InboundMessage{
					&rwp.InboundMessage{
						States: []*rwp.HWCState{
							wsFromClient.RWPState,
						},
					},
				}

				stateAsJsonString, _ := json.Marshal(wsFromClient.RWPState)

				pbdata, err := proto.Marshal(incomingMessages[0])
				log.Should(err)
				header := make([]byte, 4)                                  // Create a 4-bytes header
				binary.LittleEndian.PutUint32(header, uint32(len(pbdata))) // Fill it in
				pbdata = append(header, pbdata...)

				BroadcastMessage(&wsToClient{
					RWPASCIIToPanel:    strings.Join(helpers.InboundMessagesToRawPanelASCIIstrings(incomingMessages), "\n"),
					RWPJSONToPanel:     string(stateAsJsonString),
					RWPProtobufToPanel: prettyHexPrint(pbdata),
				})

				incoming <- incomingMessages
				//log.Println(log.Indent(incomingMessages))
			}

			if wsFromClient.ImageMode != "" {
				stopDemos()
				incomingMessages := []*rwp.InboundMessage{
					&rwp.InboundMessage{
						States: []*rwp.HWCState{},
					},
				}

				for _, HWCID := range wsFromClient.Image_HWCIDs {
					dispInfo := getDisplay(uint32(HWCID))
					if dispInfo != nil && dispInfo.Type != "text" {

						//file, err := os.ReadFile("RGB-64x32.png")
						file, err := createTestImage(dispInfo.W, dispInfo.H, wsFromClient.ImageMode)
						log.Must(err)

						if len(wsFromClient.ImageData) > 0 {
							file = wsFromClient.ImageData
						}

						// Custom WxH:
						width := dispInfo.W
						height := dispInfo.H

						// Specific scaling
						scalingValue := rwp.ProcGfxConverter_STRETCH

						// Specific encoding
						encodingValue := rwp.ProcGfxConverter_ImageTypeE(0)
						switch wsFromClient.ImageMode {
						case "color":
							encodingValue = rwp.ProcGfxConverter_RGB16bit
						case "gray":
							encodingValue = rwp.ProcGfxConverter_Gray4bit
						}

						state := &rwp.HWCState{
							HWCIDs: []uint32{uint32(HWCID)},
							Processors: &rwp.Processors{
								GfxConv: &rwp.ProcGfxConverter{
									W:         uint32(width),
									H:         uint32(height),
									ImageType: encodingValue,
									Scaling:   scalingValue,
									ImageData: file,
								},
							},
						}

						// log.Println(log.Indent(state))
						rawpanelproc.StateProcessor(state)

						incomingMessages[0].States = append(incomingMessages[0].States, state)

						if len(wsFromClient.Image_HWCIDs) == 1 {
							stateAsJsonString, _ := json.Marshal(state)
							BroadcastMessage(&wsToClient{
								RWPASCIIToPanel:    strings.Join(helpers.InboundMessagesToRawPanelASCIIstrings(incomingMessages), "\n"),
								RWPJSONToPanel:     string(stateAsJsonString),
								RWPProtobufToPanel: "(not rendered)",
							})
						}
					}
				}
				incoming <- incomingMessages
			}
		}
	}
}

func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/logoheader.png", logoheaderPage)
	http.HandleFunc("/kasperwashere.png", kasperwasherePage)

	http.HandleFunc("/vanilla-picker.js", colorPickerJS)

	http.HandleFunc("/panel", panelPage)
	http.HandleFunc("/ws", wsEndpoint)
}
