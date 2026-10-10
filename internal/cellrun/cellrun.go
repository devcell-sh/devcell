// Package cellrun tracks agent sessions: when a cell started, which tool
// ran, and when it stopped. Records persist as JSON in the project directory.
package cellrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Record is one cell run, from Begin to its recorded stop.
type Record struct {
	Started time.Time  `json:"started"`
	Stopped *time.Time `json:"stopped"`
	Clean   *bool      `json:"clean"`
	Tool    string     `json:"tool"`
	Args    []string   `json:"args"`
}

// Begin starts and persists a new Record for a cell run in projectDir.
func Begin(projectDir, tool string, args []string) (*Record, error) {
	if args == nil {
		args = []string{}
	}
	rec := &Record{
		Started: time.Now().UTC(),
		Tool:    tool,
		Args:    args,
	}
	if err := writeRecord(projectDir, rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func (r *Record) Finish(projectDir string, waitErr error) error {
	now := time.Now().UTC()
	r.Stopped = &now
	clean := waitErr == nil
	r.Clean = &clean
	return writeRecord(projectDir, r)
}

func writeRecord(projectDir string, rec *Record) error {
	dir := filepath.Join(projectDir, ".devcell")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "cell.json.tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "cell.json"))
}
