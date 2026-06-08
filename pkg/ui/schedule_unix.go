// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

//go:build !windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// cronMarker tags a crontab line so knoxctl can find and manage its own entries.
const cronMarker = "# knoxctl-job:"

// applySchedule installs (or refreshes) a cron entry for the job. It first
// removes any existing entry for the same ID, then appends the new one.
func applySchedule(job ScheduledJob) error {
	expr, err := cronExpr(job.Schedule)
	if err != nil {
		return err
	}
	self := resolveKnoxctl()
	logFile := cronLogPath(job.ID)
	line := fmt.Sprintf("%s %s schedule run %s >> %s 2>&1 %s%s",
		expr, shquote(self), job.ID, shquote(logFile), cronMarker, job.ID)

	lines := currentCrontab()
	lines = dropJobLines(lines, job.ID)
	lines = append(lines, line)
	return writeCrontab(lines)
}

// removeSchedule deletes the job's cron entry, if any.
func removeSchedule(id string) error {
	lines := currentCrontab()
	filtered := dropJobLines(lines, id)
	if len(filtered) == len(lines) {
		return nil // nothing to remove
	}
	return writeCrontab(filtered)
}

func cronLogPath(id string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	dir := home + "/.accuknox-config/logs"
	_ = os.MkdirAll(dir, 0o750) // #nosec G301
	return dir + "/" + id + ".log"
}

// currentCrontab returns the user's crontab lines (empty when none is set).
func currentCrontab() []string {
	out, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		// "no crontab for user" is reported as an error; treat as empty.
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func dropJobLines(lines []string, id string) []string {
	out := lines[:0:0]
	for _, l := range lines {
		if strings.Contains(l, cronMarker+id) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// writeCrontab replaces the user's crontab with the given lines via `crontab -`.
func writeCrontab(lines []string) error {
	if _, err := exec.LookPath("crontab"); err != nil {
		return fmt.Errorf("crontab is not available; install cron to schedule jobs")
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update crontab: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// shquote single-quotes a string for safe use in a crontab command line.
func shquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
