package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

// NetworkEvent represents a Network Event
type NetworkEvent struct {
	// Commmand that initiated the network event
	ProcessName string `json:"processName"`

	// IP address
	RemoteIP string `json:"remoteIP,omitempty"`

	// Port
	Port int32 `json:"port,omitempty"`

	// Flow of network event (egress or ingress)
	Flow string `json:"type"`

	// PID of the caller
	PID int32 `json:"pid"`

	// Network protocol
	Protocol string `json:"protocol"`
}

// NetworkCache stores the network events for processing
type NetworkCache struct {
	// Cache holds <pid>: <network-event>
	Cache map[int32][]*NetworkEvent

	// Locks
	mu sync.RWMutex
}

// NewNetworkCache instantiates the network cache
func NewNetworkCache() *NetworkCache {
	return &NetworkCache{
		Cache: make(map[int32][]*NetworkEvent),
	}
}

// AddNetworkEvent adds a new network event in cache
func (nc *NetworkCache) AddNetworkEvent(log *kaproto.Log) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if _, exists := nc.Cache[log.HostPID]; exists {
		return
	}

	event := &NetworkEvent{
		PID:         log.HostPID,
		ProcessName: getActualProcessName(log.ProcessName),
	}

	event.Flow = extractNetworkFlow(log.Data, log.Resource)
	if event.Flow == "" {
		return // only egress and ingress flows are considered
	}

	if strings.Contains(log.Data, "tcp_") {
		event.Protocol = "TCP"
		nc.handleTCPEvent(event, log.Resource)
	} else if strings.Contains(log.Data, "syscall=SYS_SOCKET") &&
		strings.Contains(log.Resource, "SOCK_DGRAM") &&
		strings.Contains(log.Resource, "domain=AF_INET") {

		fmt.Printf("handling udp")
		event.Protocol = "UDP"
	} else if strings.Contains(log.Resource, "AF_UNIX") || strings.Contains(log.Data, "domain=SYS_BIND") {
		// Exit and don't add
		return
	}

	nc.Cache[event.PID] = append(nc.Cache[event.PID], event)
}

// StartCachingEvents will cache the network log events
func (nc *NetworkCache) StartCachingEvents(logs []kaproto.Log) {
	for _, log := range logs {
		nc.AddNetworkEvent(&log)
	}
}

// handleTCPEvent handles an event if the data contains tcp
func (nc *NetworkCache) handleTCPEvent(event *NetworkEvent, data string) {
	resources := strings.Split(data, " ")

	for _, r := range resources {
		parts := strings.SplitN(r, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, val := parts[0], parts[1]

		switch key {
		case "remoteip":
			event.RemoteIP = val
		case "port":
			if portInt, err := strconv.ParseInt(val, 10, 32); err == nil {
				event.Port = int32(portInt)
			}
		case "protocol":
			event.Protocol = val
		}
	}
}

// SaveNetworkCacheJSON saves the NetworkCache data to a JSON file
func (nc *NetworkCache) SaveNetworkCacheJSON(filename string) error {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	// Create a slice to hold all network events
	var allEvents []*NetworkEvent
	for _, events := range nc.Cache {
		allEvents = append(allEvents, events...)
	}

	// Create a struct to hold the events for JSON marshaling
	data := struct {
		NetworkEvents []*NetworkEvent `json:"networkEvents"`
	}{
		NetworkEvents: allEvents,
	}

	// Marshal the data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling network cache to JSON: %v", err)
	}

	// Write the JSON data to the file
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing network cache to file: %v", err)
	}

	fmt.Printf("Network cache saved to %s\n", filename)
	return nil
}
