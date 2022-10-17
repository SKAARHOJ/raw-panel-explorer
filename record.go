package main

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"

	rwp "github.com/SKAARHOJ/rawpanel-lib/ibeam_rawpanel"
	topology "github.com/SKAARHOJ/rawpanel-lib/topology"
	log "github.com/s00500/env_logger"
)

type PanelInfo struct {
	Name         string
	Model        string
	Serial       string
	IDDisplayHWC int
	DisplayHWCs  []uint32

	TopologyJSON string
	TopologyData *topology.Topology
	TopologySVG  string

	BaseModel string
	Variant   string
}

type TriggerRecording struct {
	Panels   []TriggerRecordingPanel
	Triggers []*TriggerRecordingTrigger

	fileName string
	time     time.Time

	isRecording atomic.Bool

	sync.RWMutex
}

type TriggerRecordingPanel struct {
	Port         int
	Serial       string
	ProtocolMode uint16

	PanelInfo *PanelInfo
}

type TriggerRecordingTrigger struct {
	Event    *rwp.HWCEvent
	PanelIdx int
	Time     time.Duration
}

func (tr *TriggerRecording) recordTriggerForPanel(event *rwp.HWCEvent, idx int) {
	tr.Lock()
	defer tr.Unlock()

	addEvent := &TriggerRecordingTrigger{
		Event:    event,
		PanelIdx: idx,
		Time:     time.Now().Sub(tr.time),
	}
	tr.Triggers = append(tr.Triggers, addEvent)
	log.Println("Adding event", event, "for panel idx", idx, "with timing", time.Now().Sub(tr.time))
}

// Initialize recording
func (tr *TriggerRecording) InitRecording(fileName string) error {
	tr.Lock()
	defer tr.Unlock()

	tr.fileName = fileName
	tr.time = time.Now()

	addPanel := TriggerRecordingPanel{
		Port:         0,
		Serial:       "",
		ProtocolMode: 0,

		PanelInfo: &PanelInfo{},
	}
	tr.Panels = []TriggerRecordingPanel{addPanel}

	err := tr.saveToFile()
	if log.Should(err) {
		return err
	}

	log.Println("Recording initiated to file", tr.fileName)
	tr.isRecording.Store(true)
	return nil
}

func (tr *TriggerRecording) SetTopologyJSON(topologyJSON string, topologyData *topology.Topology) {
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].PanelInfo.TopologyData = topologyData
	tr.Panels[0].PanelInfo.TopologyJSON = topologyJSON

	tr.Panels[0].PanelInfo.DisplayHWCs = topologyData.GetHWCsWithDisplay()
}

func (tr *TriggerRecording) SetTopologySVG(topologySVG string) {
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].PanelInfo.TopologySVG = topologySVG
}

func (tr *TriggerRecording) SetModel(model string) {
	log.Println(model)
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].PanelInfo.Model = model
}

func (tr *TriggerRecording) SetSerial(serial string) {
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].PanelInfo.Serial = serial
	tr.Panels[0].Serial = serial
}

func (tr *TriggerRecording) SetName(name string) {
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].PanelInfo.Name = name
}

func (tr *TriggerRecording) SetPort(port int) {
	if !tr.isRecording.Load() {
		return
	}

	tr.Lock()
	defer tr.Unlock()

	tr.Panels[0].Port = port
}

func (tr *TriggerRecording) SaveToFile() error {
	tr.Lock()
	defer tr.Unlock()

	return tr.saveToFile()
}

func (tr *TriggerRecording) saveToFile() error {
	file, err := json.MarshalIndent(tr, "", "\t")
	if err == nil {
		err = os.WriteFile(tr.fileName, file, 0755)
		return err
	}
	return err
}
