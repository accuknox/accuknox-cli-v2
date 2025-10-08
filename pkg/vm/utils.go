package vm

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
	"github.com/go-ping/ping"
	"github.com/olekukonko/tablewriter"
	"github.com/shirou/gopsutil/host"
	gops "github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"

	"k8s.io/kubectl/pkg/util/slice"
)

var (
	green     = color.New(color.FgGreen).SprintFunc()
	red       = color.New(color.FgRed).SprintFunc()
	boldWhite = color.New(color.FgWhite, color.Bold).SprintFunc()
)

type portInfo struct {
	portNo   string
	ruleType string
}
type FirewallStatus struct {
	DefaultIncomingPolicy string
	DefaultOutgoingPolicy string
	AllowedRules          map[string]bool // Key: "direction:port"
}

// PortUsageInfo holds information about a port that is currently in use.
type PortUsageInfo struct {
	PID         int32
	ProcessName string
}

type NodeInfo struct {
	Enforcer           string
	BTF                string
	KubearmorSupported bool
	KernelVersion      string
	OsImage            string
	NodeType           onboard.NodeType
	VmMode             onboard.VMMode
}

var LsmOrder = []string{"bpf", "apparmor", "selinux"}

func GetEnforcer() (string, string, error) {
	// Detecting enforcer
	nodeEnforcer := DetectEnforcer(LsmOrder)
	if (nodeEnforcer == "apparmor") && !CheckIfApparmorFsPresent() {
		nodeEnforcer = "NA"
	}
	if nodeEnforcer == "NA" {
		logger.Info1("Node doesn't supports any KubeArmor Supported Lsm, Enforcement is disabled")
		nodeEnforcer = "none"
	}
	return nodeEnforcer, CheckBtfSupport(), nil
}

// DetectEnforcer detect the enforcer on the node
func DetectEnforcer(lsmOrder []string) string {
	supportedLsms := []string{}
	lsm := []byte{}
	lsmPath := "/sys/kernel/security/lsm"

	if _, err := os.Stat(filepath.Clean(lsmPath)); err == nil {
		lsm, err = os.ReadFile(lsmPath)
		if err != nil {
			logger.Error("Failed to read /sys/kernel/security/lsm " + err.Error())
			// goto probeLSM
			logger.Error("Failure in checking lsm file")
			goto probeLSM

		}
	}

	supportedLsms = strings.Split(string(lsm), ",")

probeLSM:

	if !slice.ContainsString(supportedLsms, "apparmor", nil) {
		apparmorModule := "/sys/module/apparmor/parameters/enabled"
		if _, err := os.Stat(filepath.Clean(apparmorModule)); err == nil {
			data, err := os.ReadFile(apparmorModule)
			if err == nil {
				status := strings.TrimSpace(string(data))
				if status == "Y" {
					supportedLsms = append(supportedLsms, "apparmor")
				} else {
					//apparmor not supported
					return ""
				}
			} else {

				logger.Error("Failed to read /sys/module/apparmor/parameters/enabled " + err.Error())
				return ""
			}
		}
	}

	return selectLsm(lsmOrder, GetAvailableLsms(), supportedLsms)
}

// selectLsm Function
func selectLsm(lsmOrder, availablelsms, supportedlsm []string) string {
	var lsm string

lsmselection:
	//check lsm preference order
	if len(lsmOrder) != 0 {
		lsm = lsmOrder[0]
		lsmOrder = lsmOrder[1:]
		if slice.ContainsString(supportedlsm, lsm, nil) && slice.ContainsString(availablelsms, lsm, nil) {
			return lsm
		}
		goto lsmselection
	}

	// fallback to available lsms order
	if len(availablelsms) != 0 {
		lsm = availablelsms[0]
		availablelsms = availablelsms[1:]
		if slice.ContainsString(supportedlsm, lsm, nil) {
			return lsm
		}
		goto lsmselection
	}

	return "NA"
}

// CheckBtfSupport checks if BTF is present
func CheckBtfSupport() string {
	btfPath := "/sys/kernel/btf/vmlinux"
	if _, err := os.Stat(filepath.Clean(btfPath)); err == nil {
		return "yes"
	}
	return "no"
}
func CheckIfApparmorFsPresent() bool {
	path := "/etc/apparmor.d/tunables"
	if _, err := os.Stat(filepath.Clean(path)); err == nil {
		return true
	}
	return false
}

// GetAvailableLsms Function
func GetAvailableLsms() []string {
	return []string{"bpf", "selinux", "apparmor"}
}

func GetKernelVersion() (string, error) {
	info, err := host.Info()
	if err != nil {
		return "", err
	}
	return info.KernelVersion, nil
}

func charsToString(ca [65]int8) string {
	s := make([]byte, 0, len(ca))
	for _, v := range ca {
		if v == 0 {
			break
		}
		s = append(s, byte(v))
	}
	return string(s)
}

func kubeArmorCompatibility() *NodeInfo {
	// get enforcer
	var err error
	var nodeInfo NodeInfo
	enforcer, btf, err := GetEnforcer()
	if err != nil {
		logger.Error("Error getting enforcer:", err)
	}
	nodeInfo.Enforcer = enforcer
	nodeInfo.BTF = btf
	// check kernel version
	nodeInfo.KernelVersion, err = GetKernelVersion()
	if err != nil {
		logger.Error("Error getting kernel version:", err)
	}
	if enforcer == "none" {
		nodeInfo.KubearmorSupported = false
	} else {
		nodeInfo.KubearmorSupported = true
	}
	info, err := host.Info()
	if err != nil {
		logger.Error("error getting host info")
	}
	nodeInfo.OsImage = info.Platform + info.PlatformVersion

	return &nodeInfo
}

func checkPorts() (map[string]string, error) {
	ports := getPortList()
	// Get all system info efficiently
	firewallStatus, err := getFirewallStatus()
	if err != nil {
		return nil, fmt.Errorf("could not get firewall status: %w", err)
	}

	listeningPorts, err := getListeningPorts()
	if err != nil {
		return nil, fmt.Errorf("could not get listening ports: %w", err)
	}

	finalStatus := make(map[string]string)
	for _, port := range ports {
		mapKey := fmt.Sprintf("Port %-5s (%s)", port.portNo, port.ruleType)

		// Determine firewall status
		if port.ruleType != "internal" {
			isBlocked := true
			firewallKey := fmt.Sprintf("%s:%s", port.ruleType, port.portNo)
			if firewallStatus.AllowedRules[firewallKey] {
				isBlocked = false
			} else if port.ruleType == "in" && firewallStatus.DefaultIncomingPolicy == "allow" {
				isBlocked = false
			} else if port.ruleType == "out" && firewallStatus.DefaultOutgoingPolicy == "allow" {
				isBlocked = false
			}

			// If blocked no need to check if in use or not
			if isBlocked {
				finalStatus[mapKey] = red("BLOCKED by firewall")
				continue
			}
		}

		// If allowed, check in use or not
		if usage, inUse := listeningPorts[port.portNo]; inUse {
			processInfo := fmt.Sprintf("PID: %d", usage.PID)
			if usage.ProcessName != "" {
				processInfo = fmt.Sprintf("PID: %d, Name: %s", usage.PID, usage.ProcessName)
			}
			finalStatus[mapKey] = fmt.Sprintf("In use by %s", processInfo)
		} else {
			finalStatus[mapKey] = green("ALLOWED (Available)")
		}
	}

	return finalStatus, nil
}
func printMapAsTable(heading []string, data map[string]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(heading)
	table.SetBorder(true)
	table.SetRowLine(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	for key, value := range data {
		table.Append([]string{key, value})
	}
	table.Render()
}

func getInstalledAgents() (map[string]string, onboard.VMMode, onboard.NodeType) {
	mp := make(map[string]string)
	installedAgents, err := onboard.CheckInstalledSystemdServices()
	if err != nil {
		logger.Error("⚠️ Error checking systemd agents: %v\n", err)
	}
	var vmMode onboard.VMMode
	var nodeType onboard.NodeType
	if len(installedAgents) > 0 {
		for _, agent := range installedAgents {
			status, err := onboard.GetSystemdServiceStatus(agent + ".service")
			if err != nil {
				logger.Error("Error getting systemd service status")
			}
			mp[agent] = status
		}
		vmMode = onboard.VMMode_Systemd
		if len(mp) > 2 {
			nodeType = onboard.NodeType_ControlPlane
		} else {
			nodeType = onboard.NodeType_WorkerNode
		}
		return mp, vmMode, nodeType
	}
	var cc onboard.ClusterConfig
	// validate docker environment
	_, err = cc.ValidateEnv()
	if err != nil {
		return nil, "", ""
	}
	installedContainers, _, err := deboard.GetInstalledObjects()
	if err != nil {
		return nil, "", ""
	}
	if len(installedContainers) > 0 {

		for key, val := range installedContainers {
			mp[key] = val.Status
		}
		vmMode = onboard.VMMode_Docker
		if len(mp) > 3 {
			nodeType = onboard.NodeType_ControlPlane
		} else {
			nodeType = onboard.NodeType_WorkerNode
		}
		return mp, vmMode, nodeType
	}

	return nil, "", ""

}
func getPortList() []portInfo {
	var portsToCheck = []portInfo{
		// Outbound rules for connecting to external SaaS services
		{portNo: "8081", ruleType: "out"}, // spire-agent to spire-server
		{portNo: "3000", ruleType: "out"}, // control plane to knox-gateway
		{portNo: "443", ruleType: "out"},  // PEA to PPS

		// Internal rules
		{portNo: "9091", ruleType: "internal"},  // spire-agent workload API endpoint
		{portNo: "9090", ruleType: "internal"},  // spire-agent health check endpoint
		{portNo: "32767", ruleType: "internal"}, // kubearmor port

		// Inbound rules for services that receive data from workers
		{portNo: "32769", ruleType: "in"}, // SIA control plane receives data
		{portNo: "32770", ruleType: "in"}, // PEA control plane receives data
		{portNo: "32771", ruleType: "in"},
	}
	return portsToCheck
}

// getFirewallStatus runs 'ufw status verbose' once and parses its entire output.
func getFirewallStatus() (*FirewallStatus, error) {
	cmd := exec.Command("sudo", "ufw", "status", "verbose")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run 'ufw status': %w", err)
	}

	status := &FirewallStatus{
		DefaultIncomingPolicy: "deny",
		DefaultOutgoingPolicy: "allow",
		AllowedRules:          make(map[string]bool),
	}

	reDefault := regexp.MustCompile(`Default:\s+(\w+)\s+\(incoming\),\s+(\w+)\s+\(outgoing\)`)
	scanner := bufio.NewScanner(&out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := reDefault.FindStringSubmatch(line); len(matches) == 3 {
			status.DefaultIncomingPolicy = matches[1]
			status.DefaultOutgoingPolicy = matches[2]
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "ALLOW" {
			ports := strings.Split(fields[0], "/")[0]
			direction := strings.ToLower(fields[2])
			port_sep := strings.Split(ports, ",") // seperate to handle cases like 80,443/tcp
			for _, port := range port_sep {
				key := fmt.Sprintf("%s:%s", direction, port)
				status.AllowedRules[key] = true
			}
		}
	}

	return status, scanner.Err()
}
func getListeningPorts() (map[string]PortUsageInfo, error) {
	listeningPorts := make(map[string]PortUsageInfo)
	conns, err := gops.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get TCP connections: %v", err)
	}

	for _, conn := range conns {
		// listening ports (LISTEN status)
		if conn.Status != "LISTEN" || conn.Laddr.Port == 0 {
			continue
		}

		portStr := fmt.Sprintf("%d", conn.Laddr.Port)
		usageInfo := PortUsageInfo{PID: conn.Pid}

		if conn.Pid > 0 {
			proc, err := process.NewProcess(conn.Pid)
			if err == nil {
				name, err := proc.Name()
				if err == nil {
					usageInfo.ProcessName = name
				}
			}
		}
		listeningPorts[portStr] = usageInfo
	}
	return listeningPorts, nil
}
func checkSAASconnectivity(o *Options) map[string]string {
	domains := []string{
		o.SpireReadyURL,
		o.SpireMetricsURL,
		o.PPSURL,
		o.KnoxGwURL,
	}
	statuses := checkDomainStatuses(domains)
	return statuses
}
func checkDomainStatuses(rawURLs []string) map[string]string {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]string)
	timeout := 5 * time.Second

	for _, raw := range rawURLs {
		wg.Add(1)

		go func(raw string) {
			defer wg.Done()

			parsed, err := url.Parse(raw)
			if err != nil {
				mu.Lock()
				results[raw] = red("Failed")
				mu.Unlock()
				return
			}

			host := parsed.Host
			if !strings.Contains(host, ":") {
				switch parsed.Scheme {
				case "https":
					host = net.JoinHostPort(parsed.Host, "443")
				case "http":
					host = net.JoinHostPort(parsed.Host, "80")
				default:
					host = net.JoinHostPort(parsed.Host, "80") // Fallback to 80
				}
			}
			conn, err := net.DialTimeout("tcp", host, timeout)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[host] = red("Failed")
			} else {
				err = conn.Close()
				if err != nil {
					logger.Error("error closing connection")
				}
				results[host] = green("Passed")
			}
		}(raw)
	}
	wg.Wait()
	return results
}
func checkNodeconnectivity(o *Options) (map[string]string, error) {

	return pingAddress(o.CPNodeAddr)
}
func pingAddress(nodeAddr string) (map[string]string, error) {
	var mp = make(map[string]string)

	//failed by default
	mp[nodeAddr] = red("Failed")

	pinger, err := ping.NewPinger(nodeAddr)
	if err != nil {
		return mp, err
	}
	pinger.Count = 4 // packet count
	pinger.Interval = time.Second
	pinger.Timeout = 5 * time.Second
	pinger.SetPrivileged(true) // privilege

	var stats *ping.Statistics
	pinger.OnFinish = func(s *ping.Statistics) {
		stats = s
	}
	err = pinger.Run()
	if err != nil {
		return mp, err
	}
	// Check if there was any packet loss. If not, return "successful".
	if stats != nil && stats.PacketLoss == 0 {
		mp[nodeAddr] = green("Passed")
		return mp, nil
	}
	// packet loss = connection failure.
	return mp, nil
}
func printNodeData(nodeData NodeInfo) {

	var data [][]string
	fmt.Println(boldWhite("Node Info:"))

	data = append(data, []string{" ", "OS Image", green(nodeData.OsImage)})
	data = append(data, []string{" ", "Kernel version", green(nodeData.KernelVersion)})
	data = append(data, []string{" ", "Enforcer", green(nodeData.Enforcer)})
	data = append(data, []string{" ", "BTF", green(nodeData.BTF)})

	if nodeData.KubearmorSupported {

		data = append(data, []string{" ", "KubeArmor Supported", green(nodeData.KubearmorSupported)})
	} else {
		data = append(data, []string{" ", "KubeArmor Supported", red(nodeData.KubearmorSupported)})
	}
	if nodeData.NodeType != "" && nodeData.VmMode != "" {
		data = append(data, []string{" ", "Node Type", green(nodeData.NodeType)})
		data = append(data, []string{" ", "Vm mode", green(nodeData.VmMode)})
	}

	renderOutputInTableWithNoBorders(data)

}

func printData(data map[string]string, heading1, heading2, title string) {
	fmt.Println("\n" + boldWhite(title))
	printMapAsTable([]string{heading1, heading2}, data)

}
func renderOutputInTableWithNoBorders(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}
