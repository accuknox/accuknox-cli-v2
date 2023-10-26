package pkg

var (
	matchLabels       = map[string]string{"app": "discovery-engine"}
	port        int64 = 9089
	targetSvc         = "discovery-engine"
	// SysProcHeader variable contains source process, destination process path, count, timestamp and status
	SysProcHeader = []string{"Src Process", "Destination Process Path", "Count", "Last Updated Time", "Status"}
	// SysFileHeader variable contains source process, destination file path, count, timestamp and status
	SysFileHeader = []string{"Src Process", "Destination File Path", "Count", "Last Updated Time", "Status"}
	// SysNwHeader variable contains protocol, command, POD/SVC/IP, Port, Namespace, and Labels
	SysNwHeader = []string{"Protocol", "Command", "POD/SVC/IP", "Port", "Namespace", "Labels", "Count", "Last Updated Time"}
	// SysBindNwHeader variable contains protocol, command, Bind Port, Bind Address, count and timestamp
	SysBindNwHeader = []string{"Protocol", "Command", "Bind Port", "Bind Address", "Count", "Last Updated Time"}
)
