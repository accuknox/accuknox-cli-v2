package pkg

const (
	ServiceName       = "discovery-engine" // Subject to change
	Port        int64 = 8090
)

var (
	matchLabels = map[string]string{"app": "discovery-engine"}
	// TODO: Add action and few other coloumns in network
	// SysProcHeader variable contains source process, destination process path, count, timestamp and status
	SysProcHeader = []string{"Src Process", "Destination Process Path", "Count", "Last Updated Time"}
	// SysFileHeader variable contains source process, destination file path, count, timestamp and status
	SysFileHeader = []string{"Src Process", "Destination File Path", "Count", "Last Updated Time"}
	// SysNwHeader variable contains protocol, command, POD/SVC/IP, Port, Namespace, and Labels
	SysNwHeader = []string{"Protocol", "Command", "POD/SVC/IP", "Port", "Count", "Last Updated Time"}
	// SysBindNwHeader variable contains protocol, command, Bind Port, Bind Address, count and timestamp
	SysBindNwHeader = []string{"Protocol", "Command", "Bind Port", "Bind Address", "Count", "Last Updated Time"}
)
