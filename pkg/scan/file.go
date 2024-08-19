package scan

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

// FileEvent holds file events taking place
type FileEvent struct {
	// The process which wrote
	ProcessName string `json:"processName"`

	// The file where write op was done
	FileName string `json:"fileName"`

	// ID of the process that did the operation
	PID int32 `json:"pid"`
}

// FileEventHandler handles file events
type FileEventHandler struct {
	// FileCache caches the file events
	FileCache map[int32][]*FileEvent `json:"fileCache"`

	// Root directory name matches all the file events falling under
	// this name
	RootDirName string

	// locks
	mu sync.RWMutex
}

func NewFileHandler(rootDirName string) *FileEventHandler {
	return &FileEventHandler{
		FileCache:   make(map[int32][]*FileEvent),
		RootDirName: rootDirName,
	}
}

func (feh *FileEventHandler) StartAddingFileEvent(logs []kaproto.Log) {
	for _, log := range logs {
		logCopy := log
		feh.AddEvent(&logCopy)
	}
}

// AddEvent looks for only write events that are taking place in any file given
// under the root directory name (which is passed as a value to flag from command line)
func (feh *FileEventHandler) AddEvent(log *kaproto.Log) {
	feh.mu.Lock()
	defer feh.mu.Unlock()

	hasReadWriteOp := func(log string) bool {
		return strings.Contains(log, "O_WRONLY") || strings.Contains(log, "O_RDWR")
	}

	if !hasReadWriteOp(log.Data) {
		return
	}

	pathComponents := strings.Split(log.Resource, "/")

	rootDirFound := false
	for _, component := range pathComponents {
		if component == feh.RootDirName {
			rootDirFound = true
			break
		}
	}

	if !rootDirFound {
		return
	}

	event := &FileEvent{
		PID:         log.HostPID,
		ProcessName: getActualProcessName(log.ProcessName),
		FileName:    log.Resource,
	}

	feh.FileCache[event.PID] = append(feh.FileCache[event.PID], event)
}

func (feh *FileEventHandler) SaveFileEventsJSON(filename string) error {
	feh.mu.RLock()
	defer feh.mu.RUnlock()

	var allEvents []*FileEvent
	for _, events := range feh.FileCache {
		allEvents = append(allEvents, events...)
	}

	data := struct {
		FileEvents []*FileEvent `json:"fileEvents"`
	}{
		FileEvents: allEvents,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling file cache to JSON: %v", err)
	}

	err = common.CleanAndWrite(filename, jsonData)
	if err != nil {
		return fmt.Errorf("error writing file cache to a file: %v", err)
	}

	return nil
}

// GenerateMarkdownTable generates a fancy markdown table of file events
func (feh *FileEventHandler) GenerateMarkdownTable() string {
	feh.mu.RLock()
	defer feh.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("| 🔢 PID | 🖥️ Process Name | 📄 File Name |\n")
	sb.WriteString("|--------|-----------------|-------------|\n")

	for _, events := range feh.FileCache {
		for _, event := range events {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n",
				event.PID,
				event.ProcessName,
				event.FileName))
		}
	}

	return sb.String()
}

// SaveFileEventMarkdown saves the file events data to a Markdown file
func (nc *NetworkCache) SaveFileEventMarkdown(filename string) error {
	markdownContent := nc.GenerateMarkdownTable()

	err := common.CleanAndWrite(filename, []byte(markdownContent))
	if err != nil {
		return fmt.Errorf("error writing network cache to markdown file: %v", err)
	}

	return nil
}
