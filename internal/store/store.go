package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/YusukeShimizu/richhistory/internal/config"
)

const (
	dirPerm      = 0o750
	filePerm     = 0o600
	eventsDir    = "events"
	eventVersion = 1
)

type Event struct {
	Version           int       `json:"version"`
	ID                string    `json:"id"`
	SessionID         string    `json:"session_id"`
	SessionName       string    `json:"session_name,omitempty"`
	Seq               int       `json:"seq"`
	Shell             string    `json:"shell"`
	ShellPID          int       `json:"shell_pid"`
	TTY               string    `json:"tty"`
	Host              string    `json:"host"`
	PWDBefore         string    `json:"pwd_before"`
	PWDAfter          string    `json:"pwd_after"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	DurationMS        int64     `json:"duration_ms"`
	ExitCode          int       `json:"exit_code"`
	Command           string    `json:"command"`
	CaptureMode       string    `json:"capture_mode"`
	StdoutText        string    `json:"stdout_text"`
	StderrText        string    `json:"stderr_text"`
	StdoutBytesTotal  int       `json:"stdout_bytes_total"`
	StderrBytesTotal  int       `json:"stderr_bytes_total"`
	StdoutStoredBytes int       `json:"stdout_stored_bytes"`
	StderrStoredBytes int       `json:"stderr_stored_bytes"`
	StdoutTruncated   bool      `json:"stdout_truncated"`
	StderrTruncated   bool      `json:"stderr_truncated"`
}

type StatusFilter string

const (
	StatusAny  StatusFilter = "any"
	StatusOK   StatusFilter = "ok"
	StatusFail StatusFilter = "fail"
)

type Filter struct {
	Limit      int
	Session    string
	CWDPrefix  string
	Status     StatusFilter
	Query      string
	QueryField string
}

type candidateFile struct {
	path string
	age  time.Time
	size int64
}

func Append(root string, cfg config.Config, event Event) error {
	if event.Version == 0 {
		event.Version = eventVersion
	}

	dir := filepath.Join(root, eventsDir)
	mkdirErr := os.MkdirAll(dir, dirPerm)
	if mkdirErr != nil {
		return fmt.Errorf("create events dir: %w", mkdirErr)
	}

	path, pathErr := eventFilePath(dir, cfg.RotateBytes, event.FinishedAt)
	if pathErr != nil {
		return pathErr
	}

	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if openErr != nil {
		return fmt.Errorf("open event file %s: %w", path, openErr)
	}
	defer func() {
		_ = file.Close()
	}()

	encoded, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return fmt.Errorf("marshal event: %w", marshalErr)
	}

	_, writeErr := file.Write(append(encoded, '\n'))
	if writeErr != nil {
		return fmt.Errorf("append event: %w", writeErr)
	}

	return nil
}

func List(root string, filter Filter) ([]Event, error) {
	events, loadErr := loadAll(root)
	if loadErr != nil {
		return nil, loadErr
	}

	matched := filterEvents(events, filter)
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].FinishedAt.After(matched[j].FinishedAt)
	})

	if filter.Limit > 0 && len(matched) > filter.Limit {
		matched = matched[:filter.Limit]
	}

	return matched, nil
}

func FindByID(root string, id string) (Event, error) {
	events, loadErr := loadAll(root)
	if loadErr != nil {
		return Event{}, loadErr
	}

	for _, event := range events {
		if event.ID == id {
			return event, nil
		}
	}

	return Event{}, fmt.Errorf("event %q not found", id)
}

func Prune(root string, cfg config.Config, now time.Time) error {
	files, total, loadErr := listCandidateFiles(filepath.Join(root, eventsDir))
	if loadErr != nil {
		return loadErr
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].age.Before(files[j].age)
	})

	totalAfterRetention, retentionErr := pruneExpired(files, total, now.AddDate(0, 0, -cfg.MaxRetentionDays))
	if retentionErr != nil {
		return retentionErr
	}

	sizeErr := pruneToBudget(files, totalAfterRetention, cfg.MaxTotalBytes)
	if sizeErr != nil {
		return sizeErr
	}

	return nil
}

func loadAll(root string) ([]Event, error) {
	dir := filepath.Join(root, eventsDir)
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read events dir: %w", readErr)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ndjson") {
			continue
		}

		names = append(names, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(names)

	events := make([]Event, 0)
	for _, name := range names {
		fileEvents, loadErr := loadEventsFromFile(name)
		if loadErr != nil {
			return nil, loadErr
		}

		events = append(events, fileEvents...)
	}

	return events, nil
}

func loadEventsFromFile(path string) ([]Event, error) {
	file, openErr := os.Open(path)
	if openErr != nil {
		return nil, fmt.Errorf("open event file %s: %w", path, openErr)
	}
	defer func() {
		_ = file.Close()
	}()

	events := make([]Event, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		unmarshalErr := json.Unmarshal(scanner.Bytes(), &event)
		if unmarshalErr != nil {
			return nil, fmt.Errorf("decode event in %s: %w", path, unmarshalErr)
		}

		events = append(events, event)
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		return nil, fmt.Errorf("scan event file %s: %w", path, scanErr)
	}

	return events, nil
}

func listCandidateFiles(dir string) ([]candidateFile, int64, error) {
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil, 0, nil
		}

		return nil, 0, fmt.Errorf("read events dir: %w", readErr)
	}

	files := make([]candidateFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ndjson") {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, 0, fmt.Errorf("stat event file %s: %w", entry.Name(), infoErr)
		}

		path := filepath.Join(dir, entry.Name())
		age := info.ModTime()
		if parsed, ok := fileDate(entry.Name()); ok {
			age = parsed
		}

		files = append(files, candidateFile{
			path: path,
			age:  age,
			size: info.Size(),
		})
		total += info.Size()
	}

	return files, total, nil
}

func pruneExpired(files []candidateFile, total int64, cutoff time.Time) (int64, error) {
	remaining := total
	for _, file := range files {
		if !file.age.Before(cutoff) {
			continue
		}

		removeErr := removeFileIfPresent(file.path)
		if removeErr != nil {
			return 0, fmt.Errorf("remove expired event file %s: %w", file.path, removeErr)
		}

		remaining -= file.size
	}

	return remaining, nil
}

func pruneToBudget(files []candidateFile, total int64, maxTotalBytes int64) error {
	if total <= maxTotalBytes {
		return nil
	}

	remaining := total
	for _, file := range files {
		statInfo, statErr := os.Stat(file.path)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}

			return statErr
		}

		removeErr := os.Remove(file.path)
		if removeErr != nil {
			return fmt.Errorf("remove event file %s: %w", file.path, removeErr)
		}

		remaining -= statInfo.Size()
		if remaining <= maxTotalBytes {
			return nil
		}
	}

	return nil
}

func removeFileIfPresent(path string) error {
	removeErr := os.Remove(path)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}

	return nil
}

func filterEvents(events []Event, filter Filter) []Event {
	matched := make([]Event, 0, len(events))
	for _, event := range events {
		if !matchesFilter(event, filter) {
			continue
		}

		matched = append(matched, event)
	}

	return matched
}

func matchesFilter(event Event, filter Filter) bool {
	if filter.Session != "" && event.SessionID != filter.Session && event.SessionName != filter.Session {
		return false
	}

	if filter.CWDPrefix != "" &&
		!strings.HasPrefix(event.PWDBefore, filter.CWDPrefix) &&
		!strings.HasPrefix(event.PWDAfter, filter.CWDPrefix) {
		return false
	}

	if filter.Status == StatusOK && event.ExitCode != 0 {
		return false
	}

	if filter.Status == StatusFail && event.ExitCode == 0 {
		return false
	}

	if filter.Query != "" && !matchesQuery(event, filter.QueryField, filter.Query) {
		return false
	}

	return true
}

func matchesQuery(event Event, field string, query string) bool {
	query = strings.ToLower(query)
	fields := []string{event.Command, event.PWDBefore, event.PWDAfter, event.StdoutText, event.StderrText}
	switch field {
	case "cmd":
		fields = []string{event.Command}
	case "cwd":
		fields = []string{event.PWDBefore, event.PWDAfter}
	case "stdout":
		fields = []string{event.StdoutText}
	case "stderr":
		fields = []string{event.StderrText}
	case "all", "":
	default:
		fields = []string{event.Command, event.PWDBefore, event.PWDAfter, event.StdoutText, event.StderrText}
	}

	for _, candidate := range fields {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}

	return false
}

func eventFilePath(dir string, rotateBytes int64, when time.Time) (string, error) {
	base := filepath.Join(dir, when.UTC().Format("2006-01-02"))
	path := base + ".ndjson"
	if rotateBytes <= 0 {
		return path, nil
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return path, nil
		}

		return "", fmt.Errorf("stat event file %s: %w", path, statErr)
	}

	if info.Size() < rotateBytes {
		return path, nil
	}

	return rotatedEventFilePath(base, rotateBytes)
}

func rotatedEventFilePath(base string, rotateBytes int64) (string, error) {
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s.%d.ndjson", base, index)
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return candidate, nil
			}

			return "", fmt.Errorf("stat rotated event file %s: %w", candidate, statErr)
		}

		if info.Size() < rotateBytes {
			return candidate, nil
		}
	}
}

func fileDate(name string) (time.Time, bool) {
	if len(name) < len("2006-01-02") {
		return time.Time{}, false
	}

	parsed, parseErr := time.Parse("2006-01-02", name[:10])
	if parseErr != nil {
		return time.Time{}, false
	}

	return parsed.UTC(), true
}
