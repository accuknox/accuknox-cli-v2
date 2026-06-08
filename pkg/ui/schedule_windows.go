// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

//go:build windows

package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

// taskName returns the Windows Task Scheduler task name for a job.
func taskName(id string) string { return "knoxctl-job-" + id }

// applySchedule creates (or replaces) a Windows scheduled task for the job.
func applySchedule(job ScheduledJob) error {
	self := resolveKnoxctl()
	// The /tr value is itself a command line; quote the exe so spaces are safe.
	run := fmt.Sprintf(`\"%s\" schedule run %s`, self, job.ID)

	args := []string{"/create", "/f", "/tn", taskName(job.ID), "/tr", run}
	sched, err := schtasksSchedule(job.Schedule)
	if err != nil {
		return err
	}
	args = append(args, sched...)

	if out, err := exec.Command("schtasks", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeSchedule deletes the job's scheduled task, if it exists.
func removeSchedule(id string) error {
	out, err := exec.Command("schtasks", "/delete", "/f", "/tn", taskName(id)).CombinedOutput()
	if err != nil {
		// Missing task is not an error for our purposes.
		if strings.Contains(strings.ToLower(string(out)), "cannot find") {
			return nil
		}
		return fmt.Errorf("failed to delete scheduled task: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// schtasksSchedule maps a Schedule onto schtasks /sc flags.
func schtasksSchedule(s Schedule) ([]string, error) {
	switch s.Frequency {
	case "hourly":
		return []string{"/sc", "HOURLY"}, nil
	case "daily":
		if _, _, err := parseHM(s.Time); err != nil {
			return nil, err
		}
		return []string{"/sc", "DAILY", "/st", s.Time}, nil
	case "weekly":
		if _, _, err := parseHM(s.Time); err != nil {
			return nil, err
		}
		if _, err := weekdayNum(s.Weekday); err != nil {
			return nil, err
		}
		return []string{"/sc", "WEEKLY", "/d", strings.ToUpper(s.Weekday), "/st", s.Time}, nil
	}
	return nil, fmt.Errorf("invalid frequency %q", s.Frequency)
}
