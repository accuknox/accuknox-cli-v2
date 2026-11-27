package probe

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	kprobe "github.com/kubearmor/kubearmor-client/probe"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DockerOptions struct {
	Output string
	GRPC   string
}

type dockerClient interface {
	onboard.DockerContainerLister
	CopyFromContainer(context.Context, string, client.CopyFromContainerOptions) (client.CopyFromContainerResult, error)
}

type dockerResult struct {
	Mode        string
	Container   string
	Host        kprobe.KubeArmorProbeData
	Policies    *pb.ProbeResponse `json:",omitempty"`
	PolicyError string            `json:",omitempty"`
}

// PrintDocker reads KubeArmor's snapshot through Docker's archive API. It does
// not execute commands inside the container or modify its filesystem.
func PrintDocker(ctx context.Context, w, warnings io.Writer, options DockerOptions) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cli, err := onboard.CreateDockerClient()
	if err != nil {
		return false, err
	}
	defer cli.Close()
	return printDocker(ctx, cli, w, warnings, options, getPolicies)
}

func printDocker(ctx context.Context, cli dockerClient, w, warnings io.Writer, options DockerOptions, policies func(context.Context, string) (*pb.ProbeResponse, error)) (bool, error) {
	agents, err := onboard.ListDockerAgents(ctx, cli)
	if err != nil {
		return false, err
	}
	var targets []container.Summary
	for _, item := range agents {
		if onboard.DockerAgentName(item) == "kubearmor" {
			targets = append(targets, item)
		}
	}
	if len(targets) == 0 {
		return false, nil
	}
	var running []container.Summary
	for _, item := range targets {
		if item.State == container.StateRunning {
			running = append(running, item)
		}
	}
	if len(running) == 0 {
		return true, fmt.Errorf("Docker KubeArmor container is not running; check docker ps -a and docker logs kubearmor")
	}
	if len(running) > 1 {
		return true, fmt.Errorf("multiple running Docker KubeArmor containers found; cannot select a unique probe target")
	}
	target := running[0]
	archive, err := cli.CopyFromContainer(ctx, target.ID, client.CopyFromContainerOptions{SourcePath: "/tmp/karmorProbeData.cfg"})
	if err != nil {
		return true, fmt.Errorf("reading KubeArmor probe snapshot from Docker: %w", err)
	}
	defer archive.Content.Close()
	host, err := readSnapshot(archive.Content)
	if err != nil {
		return true, err
	}
	name := target.ID
	if len(target.Names) > 0 {
		name = strings.TrimPrefix(target.Names[0], "/")
	}
	result := dockerResult{Mode: "docker", Container: name, Host: host}
	result.Policies, err = policies(ctx, options.GRPC)
	if err != nil {
		result.PolicyError = err.Error()
		fmt.Fprintf(warnings, "Warning: live KubeArmor policies unavailable: %v\n", err)
	}
	if options.Output == "json" {
		return true, json.NewEncoder(w).Encode(result)
	}
	fmt.Fprintf(w, "\nFound KubeArmor running in Docker mode (%s)\n\nHost :\n", name)
	printHost(w, host, options.Output == "no-color")
	if result.Policies != nil {
		printPolicies(w, result.Policies)
	}
	return true, nil
}

func readSnapshot(r io.Reader) (kprobe.KubeArmorProbeData, error) {
	var host kprobe.KubeArmorProbeData
	reader := tar.NewReader(r)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return host, fmt.Errorf("KubeArmor probe snapshot is missing from Docker archive")
		}
		if err != nil {
			return host, fmt.Errorf("reading Docker probe archive: %w", err)
		}
		if path.Base(header.Name) != "karmorProbeData.cfg" {
			continue
		}
		const maxSize = 1024 * 1024
		if !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxSize {
			return host, fmt.Errorf("invalid KubeArmor probe snapshot type or size")
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
		if err != nil {
			return host, err
		}
		if err := json.Unmarshal(data, &host); err != nil {
			return host, fmt.Errorf("parsing Docker KubeArmor probe snapshot: %w", err)
		}
		if host.OSImage == "" && host.KernelVersion == "" {
			return host, fmt.Errorf("Docker KubeArmor probe snapshot contains no host information")
		}
		return host, nil
	}
}

func getPolicies(ctx context.Context, endpoint string) (*pb.ProbeResponse, error) {
	if endpoint == "" {
		endpoint = os.Getenv("KUBEARMOR_SERVICE")
	}
	if endpoint == "" {
		endpoint = "localhost:32767"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return pb.NewProbeServiceClient(conn).GetProbeData(ctx, &emptypb.Empty{})
}

func printHost(w io.Writer, host kprobe.KubeArmorProbeData, noColor bool) {
	value := func(s string) string {
		if noColor {
			return s
		}
		return color.New(color.FgGreen).Sprint(s)
	}
	rows := [][]string{
		{" ", "OS Image:", value(host.OSImage)},
		{" ", "Kernel Version:", value(host.KernelVersion)},
		{" ", "Kubelet Version:", value(host.KubeletVersion)},
		{" ", "Container Runtime:", value(host.ContainerRuntime)},
		{" ", "Active LSM:", value(host.ActiveLSM)},
		{" ", "Host Security:", value(fmt.Sprint(host.HostSecurity))},
		{" ", "Container Security:", value(fmt.Sprint(host.ContainerSecurity))},
		{" ", "Container Default Posture:", value(host.ContainerDefaultPosture.FileAction) + "(File)", value(host.ContainerDefaultPosture.CapabilitiesAction) + "(Capabilities)", value(host.ContainerDefaultPosture.NetworkAction) + "(Network)"},
		{" ", "Host Default Posture:", value(host.HostDefaultPosture.FileAction) + "(File)", value(host.HostDefaultPosture.CapabilitiesAction) + "(Capabilities)", value(host.HostDefaultPosture.NetworkAction) + "(Network)"},
		{" ", "Host Visibility:", value(host.HostVisibility)},
	}
	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(rows)
	table.Render()
}

func printPolicies(w io.Writer, data *pb.ProbeResponse) {
	if len(data.HostMap) > 0 {
		fmt.Fprintln(w, "Host Policies :")
		table := tablewriter.NewWriter(w)
		table.SetHeader([]string{"HOST NAME", "POLICY"})
		table.SetRowLine(true)
		table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
		var hosts []string
		for name := range data.HostMap {
			hosts = append(hosts, name)
		}
		sort.Strings(hosts)
		for _, name := range hosts {
			for _, policy := range data.HostMap[name].GetPolicyList() {
				table.Append([]string{name, policy})
			}
		}
		table.Render()
	}
	fmt.Fprintln(w, "Armored Up Containers :")
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"CONTAINER NAME", "POLICY"})
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
	names := append([]string{}, data.ContainerList...)
	sort.Strings(names)
	for _, name := range names {
		info, ok := data.ContainerMap[name]
		if !ok {
			table.Append([]string{name, ""})
			continue
		}
		if info.GetPolicyEnabled() != 1 {
			continue
		}
		for _, policy := range info.GetPolicyList() {
			table.Append([]string{name, policy})
		}
	}
	table.Render()
}
