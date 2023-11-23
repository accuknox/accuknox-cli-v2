package summary

import (
	"sort"
	"sync"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
)

type Workload struct {
	mu       sync.RWMutex
	Clusters map[string]*Cluster
}

type Cluster struct {
	mu sync.RWMutex
	*summary.Cluster
	Namespaces map[string]*Namespace
}

type Namespace struct {
	mu sync.RWMutex
	*summary.Namespace
	WorkloadTypes map[string]*WorkloadType
}

type WorkloadType struct {
	Events *Events
	Labels *Labels
	*summary.WorkloadEvents
}

type Labels struct {
	mu     sync.RWMutex
	Labels string
}

type Events struct {
	mu      sync.RWMutex
	File    []*summary.ProcessFileEvent
	Process []*summary.ProcessFileEvent
	Ingress []*summary.NetworkEvent
	Egress  []*summary.NetworkEvent
	Bind    []*summary.NetworkEvent
}

// Workload methods
func (w *Workload) AddCluster(clusterName string, cluster *Cluster) *Cluster {
	w.mu.Lock()
	defer w.mu.Unlock()

	if existingCluster, ok := w.Clusters[clusterName]; ok {
		// Return existing cluster if it already exists
		return existingCluster
	}

	if w.Clusters == nil {
		w.Clusters = make(map[string]*Cluster)
	}
	w.Clusters[clusterName] = cluster
	return cluster
}

func (w *Workload) GetCluster(clusterName string) *Cluster {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if cluster, ok := w.Clusters[clusterName]; ok {
		return cluster
	}
	return nil
}

// Cluster methods
func (c *Cluster) AddNamespace(namespaceName string, namespace *Namespace) *Namespace {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existingNamespace, ok := c.Namespaces[namespaceName]; ok {
		return existingNamespace
	}

	if c.Namespaces == nil {
		c.Namespaces = make(map[string]*Namespace)
	}
	c.Namespaces[namespaceName] = namespace
	return namespace
}

func (c *Cluster) GetNamespace(namespaceName string) *Namespace {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if ns, ok := c.Namespaces[namespaceName]; ok {
		return ns
	}
	return nil
}

// Namespace
func (ns *Namespace) AddWorkloadType(workloadTypeName string, workloadType *WorkloadType) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.WorkloadTypes == nil {
		ns.WorkloadTypes = make(map[string]*WorkloadType)
	}
	ns.WorkloadTypes[workloadTypeName] = workloadType
}

func (ns *Namespace) GetWorkloadType(workloadTypeName string) *WorkloadType {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if wt, ok := ns.WorkloadTypes[workloadTypeName]; ok {
		return wt
	}
	return nil
}

func (ns *Namespace) TotalEvents() int {
	total := 0
	for _, wt := range ns.WorkloadTypes {
		total += len(wt.Events.File)
		total += len(wt.Events.Process)
		total += len(wt.Events.Ingress)
		total += len(wt.Events.Egress)
		total += len(wt.Events.Bind)
	}

	return total
}

// WorkloadType
func (wt *WorkloadType) SetEvents(events *Events) {
	wt.Events = events
}

func (wt *WorkloadType) GetEvents() *Events {
	return wt.Events
}

// Events
func (e *Events) AddFileEvent(fe *summary.ProcessFileEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.File = append(e.File, fe)
}

func (e *Events) AddProcessEvent(pfe *summary.ProcessFileEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Process = append(e.Process, pfe)
}

func (e *Events) AddIngressEvent(ie *summary.NetworkEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Ingress = append(e.Ingress, ie)
}

func (e *Events) AddEgressEvent(ee *summary.NetworkEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Egress = append(e.Egress, ee)
}

func (e *Events) AddBindEvent(be *summary.NetworkEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.Bind = append(e.Bind, be)
}

func (e *Events) TotalEvents() int {
	total := 0
	total += len(e.File)
	total += len(e.Process)
	total += len(e.Ingress)
	total += len(e.Egress)
	total += len(e.Bind)
	return total
}

// Sort by events in namespace
func sortNamespacesByEvents(cluster *Cluster) []string {
	namespaceNames := make([]string, 0, len(cluster.Namespaces))

	for name := range cluster.Namespaces {
		namespaceNames = append(namespaceNames, name)
	}

	sort.Slice(namespaceNames, func(i, j int) bool {
		ni := cluster.Namespaces[namespaceNames[i]]
		nj := cluster.Namespaces[namespaceNames[j]]
		return ni.TotalEvents() > nj.TotalEvents()
	})

	return namespaceNames
}
