package scan

import (
	"reflect"
	"testing"

	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

func TestNewFileHandler(t *testing.T) {
	rootDirName := "test-dir"
	handler := NewFileHandler(rootDirName)

	if handler == nil {
		t.Fatal("NewFileHandler returned nil")
	}

	if handler.RootDirName != rootDirName {
		t.Errorf("Expected RootDirName to be %s, got %s", rootDirName, handler.RootDirName)
	}

	if handler.FileCache == nil {
		t.Error("FileCache is nil")
	}

	if len(handler.FileCache) != 0 {
		t.Errorf("Expected empty FileCache, got %d items", len(handler.FileCache))
	}
}

func TestStartAddingFileEvent(t *testing.T) {
	handler := NewFileHandler("test-dir")
	logs := []kaproto.Log{
		{
			HostPID:     1,
			ProcessName: "/usr/bin/touch",
			Resource:    "/test-dir/file1.txt",
			Data:        "O_WRONLY",
		},
		{
			HostPID:     2,
			ProcessName: "/usr/bin/echo",
			Resource:    "/test-dir/file2.txt",
			Data:        "O_RDWR",
		},
	}

	handler.StartAddingFileEvent(logs)

	if len(handler.FileCache) != 2 {
		t.Errorf("Expected 2 items in FileCache, got %d", len(handler.FileCache))
	}

	for _, log := range logs {
		events, exists := handler.FileCache[log.HostPID]
		if !exists {
			t.Errorf("Expected event for PID %d, but not found", log.HostPID)
			continue
		}

		if len(events) != 1 {
			t.Errorf("Expected 1 event for PID %d, got %d", log.HostPID, len(events))
			continue
		}

		event := events[0]
		if event.ProcessName != getActualProcessName(log.ProcessName) {
			t.Errorf("Expected ProcessName %s, got %s", getActualProcessName(log.ProcessName), event.ProcessName)
		}
		if event.FileName != log.Resource {
			t.Errorf("Expected FileName %s, got %s", log.Resource, event.FileName)
		}
	}
}

func TestAddEvent(t *testing.T) {
	tests := []struct {
		name           string
		rootDirName    string
		log            *kaproto.Log
		expectedEvents []*FileEvent
	}{
		{
			name:        "Valid write event",
			rootDirName: "test-dir",
			log: &kaproto.Log{
				HostPID:     1,
				ProcessName: "/usr/bin/touch",
				Resource:    "/test-dir/file1.txt",
				Data:        "O_WRONLY",
			},
			expectedEvents: []*FileEvent{
				{
					PID:         1,
					ProcessName: "touch",
					FileName:    "/test-dir/file1.txt",
				},
			},
		},
		{
			name:        "Non-write event",
			rootDirName: "test-dir",
			log: &kaproto.Log{
				HostPID:     2,
				ProcessName: "/usr/bin/cat",
				Resource:    "/test-dir/file2.txt",
				Data:        "O_RDONLY",
			},
			expectedEvents: nil,
		},
		{
			name:        "Event outside root directory",
			rootDirName: "test-dir",
			log: &kaproto.Log{
				HostPID:     3,
				ProcessName: "/usr/bin/echo",
				Resource:    "/other-dir/file3.txt",
				Data:        "O_WRONLY",
			},
			expectedEvents: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewFileHandler(tt.rootDirName)
			handler.AddEvent(tt.log)

			events, exists := handler.FileCache[tt.log.HostPID]
			if !exists && tt.expectedEvents != nil {
				t.Fatalf("Expected events for PID %d, but not found", tt.log.HostPID)
			}

			if !reflect.DeepEqual(events, tt.expectedEvents) {
				t.Errorf("Expected events %+v, got %+v", tt.expectedEvents, events)
			}
		})
	}
}
