package onboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

var errRestartUnitNotLoaded = errors.New("service is not loaded")

func printAgentRestartCounts(ctx context.Context, out io.Writer, services []string, exclude map[string]bool, read func(context.Context, string) (map[string]any, error)) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "SERVICE\tRESTARTS\tSTATE"); err != nil {
		return err
	}
	var failures []error
	for _, service := range services {
		if exclude[service] || exclude[strings.TrimSuffix(service, ".service")] {
			continue
		}
		properties, err := read(ctx, service)
		count, state := "N/A", "not-loaded"
		if err != nil && !errors.Is(err, errRestartUnitNotLoaded) {
			failures = append(failures, fmt.Errorf("reading %s: %w", service, err))
			state = "error"
		} else if err == nil {
			state, _ = properties["ActiveState"].(string)
			if n, ok := properties["NRestarts"].(uint32); ok {
				count = fmt.Sprint(n)
			}
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", service, count, state); err != nil {
			return err
		}
	}
	return errors.Join(append(failures, w.Flush())...)
}
