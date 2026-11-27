package onboard

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type DockerContainerLister interface {
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
}

// DockerAgentName matches explicit container names and Compose service labels,
// including installations whose Compose project prefixes the container name.
func DockerAgentName(item container.Summary) string {
	known := GetKnownContainerMap()
	for _, name := range []string{"discover", "rabbitmq", rraName, cm.Imagescan} {
		known[name] = container.Summary{}
	}
	names := append([]string{}, item.Names...)
	names = append(names, item.Labels["com.docker.compose.service"])
	for _, name := range names {
		name = strings.TrimPrefix(name, "/")
		if _, ok := known[name]; ok {
			return name
		}
	}
	return ""
}

func ListDockerAgents(ctx context.Context, cli DockerContainerLister) ([]container.Summary, error) {
	result, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listing Docker agents: %w", err)
	}
	var agents []container.Summary
	for _, item := range result.Items {
		if DockerAgentName(item) != "" {
			agents = append(agents, item)
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		if a, b := DockerAgentName(agents[i]), DockerAgentName(agents[j]); a != b {
			return a < b
		}
		return agents[i].ID < agents[j].ID
	})
	return agents, nil
}

// PrintDockerAgentVersions lists names, tags, and states, including stopped
// agents. Long output also includes container names and immutable image IDs.
func PrintDockerAgentVersions(ctx context.Context, w io.Writer, long bool) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cli, err := CreateDockerClient()
	if err != nil {
		return false, err
	}
	defer cli.Close()
	return printDockerAgentVersions(ctx, cli, w, long)
}

func printDockerAgentVersions(ctx context.Context, cli DockerContainerLister, w io.Writer, long bool) (bool, error) {
	agents, err := ListDockerAgents(ctx, cli)
	if err != nil || len(agents) == 0 {
		return false, err
	}
	fmt.Fprintln(w, "Docker agent versions:")
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if long {
		fmt.Fprintln(table, "AGENT\tCONTAINER\tIMAGE\tIMAGE ID\tSTATE")
	} else {
		fmt.Fprintln(table, "NAME\tVERSION\tSTATUS")
	}
	for _, item := range agents {
		if !long {
			fmt.Fprintf(table, "%s\t%s\t%s\n", DockerAgentName(item), AgentImageVersion(item.Image), item.State)
			continue
		}
		name := item.ID
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", DockerAgentName(item), name, item.Image, item.ImageID, item.State)
	}
	return true, table.Flush()
}
