package scan

import (
	"reflect"
	"testing"

	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

func TestAddNetworkEvent(t *testing.T) {
	tests := []struct {
		name     string
		log      kaproto.Log
		expected *NetworkEvent
	}{
		{
			name: "TCP Connect Event",
			log: kaproto.Log{
				HostPID:     24575,
				ProcessName: "/usr/bin/python3.10",
				Data:        "kprobe=tcp_connect domain=AF_INET",
				Resource:    "remoteip=127.0.0.1 port=12345 protocol=TCP",
			},
			expected: &NetworkEvent{
				PID:         24575,
				ProcessName: "python3.10",
				Flow:        "egress",
				Protocol:    "TCP",
				RemoteIP:    "127.0.0.1",
				Port:        12345,
			},
		},
		{
			name: "TCP Accept Event",
			log: kaproto.Log{
				HostPID:     24574,
				ProcessName: "/usr/bin/python3.10",
				Data:        "kprobe=tcp_accept domain=AF_INET",
				Resource:    "remoteip=127.0.0.1 port=12345 protocol=TCP",
			},
			expected: &NetworkEvent{
				PID:         24574,
				ProcessName: "python3.10",
				Flow:        "ingress",
				Protocol:    "TCP",
				RemoteIP:    "127.0.0.1",
				Port:        12345,
			},
		},
		{
			name: "UDP Socket Event",
			log: kaproto.Log{
				HostPID:     24542,
				ProcessName: "/usr/bin/python3.10",
				Data:        "syscall=SYS_SOCKET",
				Resource:    "domain=AF_INET type=SOCK_DGRAM|SOCK_CLOEXEC protocol=0",
			},
			expected: &NetworkEvent{
				PID:         24542,
				ProcessName: "python3.10",
				Flow:        "egress",
				Protocol:    "UDP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := NewNetworkCache()
			nc.AddNetworkEvent(&tt.log)

			if tt.expected == nil {
				if len(nc.Cache) != 0 {
					t.Fatalf("expected 0 events in cache, got %d", len(nc.Cache))
				}
				return
			}

			if len(nc.Cache) != 1 {
				t.Fatalf("expected 1 event in cache, got %d", len(nc.Cache))
			}

			events, exists := nc.Cache[tt.expected.PID]
			if !exists {
				t.Fatalf("expected event with PID %d to exist in cache", tt.expected.PID)
			}

			if len(events) != 1 {
				t.Fatalf("expected 1 event for PID %d, got %d", tt.expected.PID, len(events))
			}

			event := events[0]
			if !reflect.DeepEqual(event, tt.expected) {
				t.Errorf("expected event %+v, got %+v", tt.expected, event)
			}
		})
	}
}

func TestHandleNetworkEvent(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected *NetworkEvent
	}{
		{
			name: "TCP Event",
			data: "remoteip=192.168.1.1 port=8080 protocol=TCP",
			expected: &NetworkEvent{
				RemoteIP: "192.168.1.1",
				Port:     8080,
				Protocol: "TCP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &NetworkEvent{}
			nc := NewNetworkCache()
			nc.handleNetworkEvent(event, tt.data)

			if !reflect.DeepEqual(event, tt.expected) {
				t.Errorf("expected event %+v, got %+v", tt.expected, event)
			}
		})
	}
}
