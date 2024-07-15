package scan

// Client is an interface for scan subcommand's functions
type Client interface {
	// ConnectToGRPC to connects to kubearmor via exposed gRPC endpoint
	ConnectToGRPC() error
}
