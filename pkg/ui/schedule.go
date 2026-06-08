// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/bomgen"
)

// Schedule is a simple, cross-platform recurrence describing when a job runs.
// It maps cleanly onto both cron (Linux) and Task Scheduler (Windows).
type Schedule struct {
	Frequency string `json:"frequency" yaml:"frequency"` // "hourly" | "daily" | "weekly"
	Time      string `json:"time"      yaml:"time"`      // "HH:MM" for daily/weekly
	Weekday   string `json:"weekday"   yaml:"weekday"`   // "MON".."SUN" for weekly
}

// ScheduledJob is a persisted, OS-scheduled BOM generation + publish task.
type ScheduledJob struct {
	ID       string            `json:"id"       yaml:"id"`
	Name     string            `json:"name"     yaml:"name"`
	Type     string            `json:"type"     yaml:"type"` // "sbom"
	Schedule Schedule          `json:"schedule" yaml:"schedule"`
	Paused   bool              `json:"paused"   yaml:"paused"`
	SBOM     bomgen.SBOMParams `json:"sbom"     yaml:"sbom"`
	Git      gitReq            `json:"git"      yaml:"git"`
	Publish  BOMSettings       `json:"publish"  yaml:"publish"`

	LastRun    string `json:"lastRun"    yaml:"last_run"`
	LastStatus string `json:"lastStatus" yaml:"last_status"`
}

// jobMu guards read-modify-write cycles on the jobs list in the config file.
var jobMu sync.Mutex

func listJobs() []ScheduledJob { return loadAppConfig().Jobs }

// ListJobs returns all persisted scheduled jobs (exported for the CLI).
func ListJobs() []ScheduledJob { return listJobs() }

func getJob(id string) (ScheduledJob, bool) {
	for _, j := range loadAppConfig().Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return ScheduledJob{}, false
}

// upsertJob inserts or replaces a job (assigning an ID on insert) and persists it.
func upsertJob(job ScheduledJob) (ScheduledJob, error) {
	jobMu.Lock()
	defer jobMu.Unlock()
	cfg := loadAppConfig()
	if job.ID == "" {
		job.ID = newJobID()
		cfg.Jobs = append(cfg.Jobs, job)
	} else {
		found := false
		for i := range cfg.Jobs {
			if cfg.Jobs[i].ID == job.ID {
				// Preserve run bookkeeping across edits.
				job.LastRun = cfg.Jobs[i].LastRun
				job.LastStatus = cfg.Jobs[i].LastStatus
				cfg.Jobs[i] = job
				found = true
				break
			}
		}
		if !found {
			cfg.Jobs = append(cfg.Jobs, job)
		}
	}
	if err := saveAppConfig(cfg); err != nil {
		return ScheduledJob{}, err
	}
	return job, nil
}

func deleteJobFromConfig(id string) error {
	jobMu.Lock()
	defer jobMu.Unlock()
	cfg := loadAppConfig()
	out := cfg.Jobs[:0]
	for _, j := range cfg.Jobs {
		if j.ID != id {
			out = append(out, j)
		}
	}
	cfg.Jobs = out
	return saveAppConfig(cfg)
}

func setJobPaused(id string, paused bool) (ScheduledJob, error) {
	jobMu.Lock()
	defer jobMu.Unlock()
	cfg := loadAppConfig()
	for i := range cfg.Jobs {
		if cfg.Jobs[i].ID == id {
			cfg.Jobs[i].Paused = paused
			if err := saveAppConfig(cfg); err != nil {
				return ScheduledJob{}, err
			}
			return cfg.Jobs[i], nil
		}
	}
	return ScheduledJob{}, fmt.Errorf("job %q not found", id)
}

func recordJobRun(id, status string) {
	jobMu.Lock()
	defer jobMu.Unlock()
	cfg := loadAppConfig()
	for i := range cfg.Jobs {
		if cfg.Jobs[i].ID == id {
			cfg.Jobs[i].LastRun = time.Now().UTC().Format(time.RFC3339)
			cfg.Jobs[i].LastStatus = status
			_ = saveAppConfig(cfg)
			return
		}
	}
}

// newJobID returns a short, unique, filesystem/scheduler-safe job ID.
func newJobID() string {
	return "job" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func validateJob(j *ScheduledJob) error {
	if strings.TrimSpace(j.Name) == "" {
		return fmt.Errorf("job name is required")
	}
	if j.Type == "" {
		j.Type = "sbom"
	}
	if j.Type != "sbom" {
		return fmt.Errorf("unsupported job type %q", j.Type)
	}
	if strings.TrimSpace(j.SBOM.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if j.SBOM.Format == "" {
		j.SBOM.Format = "cyclonedx-json"
	}
	switch j.Schedule.Frequency {
	case "hourly":
	case "daily", "weekly":
		if _, _, err := parseHM(j.Schedule.Time); err != nil {
			return err
		}
		if j.Schedule.Frequency == "weekly" {
			if _, err := weekdayNum(j.Schedule.Weekday); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("schedule frequency must be hourly, daily, or weekly")
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Headless run (generate + publish) — invoked by the OS scheduler via the CLI.
// ──────────────────────────────────────────────────────────────────────────────

// RunJobByID generates and publishes the BOM for the given job. It is exported so
// the `knoxctl schedule run <id>` command (launched by cron / Task Scheduler) can
// call it. Progress is written to logw.
func RunJobByID(ctx context.Context, id string, logw io.Writer) error {
	job, ok := getJob(id)
	if !ok {
		return fmt.Errorf("scheduled job %q not found", id)
	}
	fmt.Fprintf(logw, "Running job %q (%s)\n", job.Name, job.ID)

	data, err := generateSBOMBytes(ctx, job.SBOM, job.Git, logw)
	if err == nil {
		_, _, err = publishBOMBytes(ctx, job.Publish, "sbom", data)
	}

	status := "success"
	if err != nil {
		status = "error: " + err.Error()
	}
	recordJobRun(id, status)
	fmt.Fprintf(logw, "Job %q finished: %s\n", job.ID, status)
	return err
}

// generateSBOMBytes runs an SBOM scan synchronously and returns the rebranded
// output. It mirrors handleSBOM but without the SSE streaming layer.
func generateSBOMBytes(ctx context.Context, params bomgen.SBOMParams, g gitReq, logw io.Writer) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "knoxctl-job-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	outFile := filepath.Join(tmpDir, "sbom.json")

	if params.Scheme == "git" {
		dir, err := cloneGitSource(ctx, params.Source, g, tmpDir, logw)
		if err != nil {
			return nil, err
		}
		params.Source = dir
		params.Scheme = ""
	}

	args := bomgen.SBOMArgs(params, outFile)
	cmd := exec.CommandContext(ctx, resolveKnoxctl(), args...) // #nosec G204
	cmd.Stdout = logw
	cmd.Stderr = logw
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("BOM generation failed: %w", err)
	}

	data, err := os.ReadFile(outFile) // #nosec G304 — path inside os.MkdirTemp dir
	if err != nil {
		return nil, fmt.Errorf("BOM generation produced no output: %w", err)
	}
	if params.UsesDepth() {
		return bomgen.StripDepthBranding(data, params.Format), nil
	}
	return rebrandSBOM(data), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP handlers
// ──────────────────────────────────────────────────────────────────────────────

func (s *Server) registerScheduleRoutes() {
	s.mux.HandleFunc("/api/schedule/jobs", cors(s.handleScheduleJobs))
	s.mux.HandleFunc("/api/schedule/save", cors(s.handleScheduleSave))
	s.mux.HandleFunc("/api/schedule/delete", cors(s.handleScheduleDelete))
	s.mux.HandleFunc("/api/schedule/pause", cors(s.handleSchedulePause))
	s.mux.HandleFunc("/api/schedule/run", cors(s.handleScheduleRun))
}

func (s *Server) handleScheduleJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"jobs": listJobs()})
}

func (s *Server) handleScheduleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var job ScheduledJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeErr(w, "invalid request: "+err.Error())
		return
	}
	if err := validateJob(&job); err != nil {
		writeErr(w, err.Error())
		return
	}
	saved, err := upsertJob(job)
	if err != nil {
		writeErr(w, "failed to save job: "+err.Error())
		return
	}
	if err := reconcileSchedule(saved); err != nil {
		writeErr(w, "job saved but scheduling failed: "+err.Error())
		return
	}
	writeJSON(w, saved)
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeErr(w, "id is required")
		return
	}
	_ = removeSchedule(req.ID)
	if err := deleteJobFromConfig(req.ID); err != nil {
		writeErr(w, "failed to delete job: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleSchedulePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     string `json:"id"`
		Paused bool   `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeErr(w, "id is required")
		return
	}
	job, err := setJobPaused(req.ID, req.Paused)
	if err != nil {
		writeErr(w, err.Error())
		return
	}
	if err := reconcileSchedule(job); err != nil {
		writeErr(w, "state saved but scheduling failed: "+err.Error())
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleScheduleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeErr(w, "id is required")
		return
	}
	if _, ok := getJob(req.ID); !ok {
		writeErr(w, "job not found")
		return
	}
	// Run asynchronously so the request returns immediately.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		_ = RunJobByID(ctx, req.ID, os.Stderr)
	}()
	writeJSON(w, map[string]string{"status": "started"})
}

// reconcileSchedule registers (or, when paused, removes) the OS-level schedule
// so it matches the job's current state.
func reconcileSchedule(job ScheduledJob) error {
	if job.Paused {
		return removeSchedule(job.ID)
	}
	return applySchedule(job)
}

// ──────────────────────────────────────────────────────────────────────────────
// Schedule helpers (shared by the platform-specific registration code)
// ──────────────────────────────────────────────────────────────────────────────

func parseHM(s string) (hour, min int, err error) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("time must be HH:MM")
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in time %q", s)
	}
	min, err = strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid minute in time %q", s)
	}
	return hour, min, nil
}

// weekdayNum maps a weekday abbreviation to its cron number (0=Sunday..6=Saturday).
func weekdayNum(wd string) (int, error) {
	switch strings.ToUpper(strings.TrimSpace(wd)) {
	case "SUN":
		return 0, nil
	case "MON":
		return 1, nil
	case "TUE":
		return 2, nil
	case "WED":
		return 3, nil
	case "THU":
		return 4, nil
	case "FRI":
		return 5, nil
	case "SAT":
		return 6, nil
	}
	return 0, fmt.Errorf("invalid weekday %q (use MON..SUN)", wd)
}

// cronExpr builds a 5-field cron expression for a schedule.
func cronExpr(s Schedule) (string, error) {
	switch s.Frequency {
	case "hourly":
		return "0 * * * *", nil
	case "daily":
		h, m, err := parseHM(s.Time)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d %d * * *", m, h), nil
	case "weekly":
		h, m, err := parseHM(s.Time)
		if err != nil {
			return "", err
		}
		d, err := weekdayNum(s.Weekday)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d %d * * %d", m, h, d), nil
	}
	return "", fmt.Errorf("invalid frequency %q", s.Frequency)
}
