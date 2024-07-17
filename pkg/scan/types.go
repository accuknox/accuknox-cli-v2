package scan

// ScanOptions primarily is used for flags for the `scan` subcommand
type ScanOptions struct {
	FilterEventType FilterEventType
	FilterEvents    FilterEvents
	GRPC            string
	Output          string
	ShowProcessTree bool
}

// Filter provides the basic filters for collection of data
type FilterEventType struct {
	System bool
	All    bool
}

// FilterEvents will filter events coming from KubeArmor
type FilterEvents struct {
	Network bool
	Process bool
	File    bool
}
