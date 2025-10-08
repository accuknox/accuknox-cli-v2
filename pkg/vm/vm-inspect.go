package vm

import (
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

type Options struct {
	SpireReadyURL   string
	SpireMetricsURL string
	PPSURL          string
	KnoxGwURL       string
	CPNodeAddr      string
}

func InspectVM(o *Options) error {

	var (
		kaCompatible      *NodeInfo
		portsAvailability map[string]string
		installedAgents   map[string]string
		vmMode            onboard.VMMode
		nodeType          onboard.NodeType
		wg                sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		kaCompatible = kubeArmorCompatibility()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ports, err := checkPorts()
		if err != nil {
			logger.Error("error checking ports", err)
			return
		}
		portsAvailability = ports
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		agents, vm, node := getInstalledAgents()
		installedAgents = agents
		vmMode = vm
		nodeType = node
	}()

	wg.Wait()

	if vmMode != "" && nodeType != "" && kaCompatible != nil {
		kaCompatible.NodeType = nodeType
		kaCompatible.VmMode = vmMode
	}

	printNodeData(*kaCompatible)
	printData(portsAvailability, "PORTS", "STATUS", "Ports Availability")
	if len(installedAgents) > 0 {
		printData(installedAgents, "AGENTS", "STATUS", "Accuknox Agents")
	} else {
		// if there are no agents installed , check connectivity.
		if o.CPNodeAddr == "" {
			// treat it as control plane node
			output := checkSAASconnectivity(o)
			printData(output, "URL", "STATUS", "Accuknox Connectivity")
		} else {
			// it's a worker-node and check connectivity to contol plane node'
			output, err := checkNodeconnectivity(o)
			if err != nil {
				logger.Error("Error checking controlplane connectivity", err.Error())
			}
			printData(output, "Node Address", "STATUS", "Control Plane Connectivity")
		}
	}

	return nil

}
