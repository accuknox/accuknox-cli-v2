package summary

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
)

// Workload aligns with the top-level data structure
type Workload struct {
	mu       sync.RWMutex        `json:"-"`
	Clusters map[string]*Cluster `json:"clusters"`
	Hash     string              `json:"-"`
}

// Cluster mirrors the protobuf Cluster structure
type Cluster struct {
	mu               sync.RWMutex `json:"-"`
	ClusterName      string       `json:"-"`
	*summary.Cluster `json:"-"`
	Namespaces       map[string]*Namespace `json:"namespaces"`
	Hash             string                `json:"-"`
}

// Namespace reflects the protobuf Namespace structure
type Namespace struct {
	mu            sync.RWMutex               `json:"-"`
	NamespaceName string                     `json:"-"`
	Deployments   map[string]*WorkloadEvents `json:"deployments"`
	ReplicaSets   map[string]*WorkloadEvents `json:"replicaSets"`
	StatefulSets  map[string]*WorkloadEvents `json:"statefulSets"`
	DaemonSets    map[string]*WorkloadEvents `json:"daemonSets"`
	Jobs          map[string]*WorkloadEvents `json:"jobs"`
	CronJobs      map[string]*WorkloadEvents `json:"cronJobs"`
	Hash          string                     `json:"-"`
}

// WorkloadEvents aligns with the protobuf WorkloadEvents
type WorkloadEvents struct {
	mu           sync.RWMutex `json:"-"`
	WorkloadName string       `json:"-"`
	Labels       string       `json:"labels"`
	Events       *Events      `json:"events"`
	Hash         string       `json:"-"`
}

// Events reflects the event structure in the protobuf
type Events struct {
	mu      sync.RWMutex                `json:"-"`
	File    []*summary.ProcessFileEvent `json:"file"`
	Process []*summary.ProcessFileEvent `json:"process"`
	Ingress []*summary.NetworkEvent     `json:"ingress"`
	Egress  []*summary.NetworkEvent     `json:"egress"`
	Bind    []*summary.NetworkEvent     `json:"bind"`
	Hash    string                      `json:"-"`
}

// All this code needs to get refactored, its very hacky.
// Workload methods
func (w *Workload) AddCluster(clusterName string, cluster *Cluster) *Cluster {
	w.mu.Lock()
	defer w.mu.Unlock()

	if existingCluster, ok := w.Clusters[clusterName]; ok {
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

func (w *Workload) GetAllClusters() []*Cluster {
	w.mu.RLock()
	defer w.mu.RUnlock()

	clusters := make([]*Cluster, 0, len(w.Clusters))
	for _, cluster := range w.Clusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

func (w *Workload) SetHash() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	hash, err := ComputeHash(w)
	if err != nil {
		return err
	}
	w.Hash = hash
	return nil
}

func (w *Workload) GetHash() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.Hash
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

func (c *Cluster) GetAllNamespaces() []*Namespace {
	c.mu.Lock()
	defer c.mu.Unlock()

	namespaces := make([]*Namespace, 0, len(c.Namespaces))
	for _, ns := range c.Namespaces {
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

func (c *Cluster) SetHash() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash, err := ComputeHash(c)
	if err != nil {
		return err
	}
	c.Hash = hash
	return nil
}

func (c *Cluster) GetHash() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Hash
}

// Namespace methods adds
func (ns *Namespace) AddDeploymentEvents(deploymentName string, deploymentEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.Deployments == nil {
		ns.Deployments = make(map[string]*WorkloadEvents)
	}
	ns.Deployments[deploymentName] = deploymentEvents
}

func (ns *Namespace) AddReplicaSetEvents(replicaSetName string, replicaSetEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.ReplicaSets == nil {
		ns.ReplicaSets = make(map[string]*WorkloadEvents)
	}
	ns.ReplicaSets[replicaSetName] = replicaSetEvents
}

func (ns *Namespace) AddStatefulSetEvents(statefulSetName string, statefulSetEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.StatefulSets == nil {
		ns.StatefulSets = make(map[string]*WorkloadEvents)
	}
	ns.StatefulSets[statefulSetName] = statefulSetEvents
}

func (ns *Namespace) AddDaemonSetEvents(daemonSetName string, daemonSetEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.DaemonSets == nil {
		ns.DaemonSets = make(map[string]*WorkloadEvents)
	}
	ns.DaemonSets[daemonSetName] = daemonSetEvents
}

func (ns *Namespace) AddJobEvents(jobName string, jobEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.Jobs == nil {
		ns.Jobs = make(map[string]*WorkloadEvents)
	}
	ns.Jobs[jobName] = jobEvents
}

func (ns *Namespace) AddCronJobEvents(cronJobName string, cronJobEvents *WorkloadEvents) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.CronJobs == nil {
		ns.CronJobs = make(map[string]*WorkloadEvents)
	}
	ns.CronJobs[cronJobName] = cronJobEvents
}

func (ns *Namespace) SetHash() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	hash, err := ComputeHash(ns)
	if err != nil {
		return err
	}
	ns.Hash = hash
	return nil
}

// Gets
func (ns *Namespace) GetDeploymentEvents(deploymentName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.Deployments[deploymentName]
}

func (ns *Namespace) GetReplicaSetEvents(replicaSetName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.ReplicaSets[replicaSetName]
}

func (ns *Namespace) GetStatefulSetEvents(statefulSetName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.StatefulSets[statefulSetName]
}

func (ns *Namespace) GetDaemonSetEvents(daemonSetName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.DaemonSets[daemonSetName]
}

func (ns *Namespace) GetJobEvents(jobName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.Jobs[jobName]
}

func (ns *Namespace) GetCronJobEvents(cronJobName string) *WorkloadEvents {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.CronJobs[cronJobName]
}

func (ns *Namespace) GetHash() string {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	return ns.Hash
}

// WorkloadEvents methods
func (we *WorkloadEvents) SetHash() error {
	we.mu.Lock()
	defer we.mu.Unlock()

	hash, err := ComputeHash(we)
	if err != nil {
		return err
	}
	we.Hash = hash
	return nil
}

func (we *WorkloadEvents) GetHash() string {
	we.mu.Lock()
	defer we.mu.Unlock()

	return we.Hash
}

func (ns *Namespace) TotalEvents() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	total := 0
	for _, we := range ns.Deployments {
		total += we.Events.TotalEvents()
	}
	for _, we := range ns.ReplicaSets {
		total += we.Events.TotalEvents()
	}
	for _, we := range ns.StatefulSets {
		total += we.Events.TotalEvents()
	}
	for _, we := range ns.DaemonSets {
		total += we.Events.TotalEvents()
	}
	for _, we := range ns.Jobs {
		total += we.Events.TotalEvents()
	}
	for _, we := range ns.CronJobs {
		total += we.Events.TotalEvents()
	}
	return total
}

// WorkloadEvents methods
func (we *WorkloadEvents) SetEvents(events *Events) {
	we.mu.Lock()
	defer we.mu.Unlock()

	we.Events = events
}

func (we *WorkloadEvents) GetEvents() *Events {
	we.mu.RLock()
	defer we.mu.RUnlock()

	return we.Events
}

// Events methods
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

func (e *Events) SetHash() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hash, err := ComputeHash(e)
	if err != nil {
		return err
	}
	e.Hash = hash
	return nil
}

func (e *Events) GetHash() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.Hash
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

func ComputeHash(v interface{}) (string, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash), nil
}
