package jobs

import (
	"os"
	"strings"
	"testing"
)

func TestWorkerRetentionQueriesMatchMigratedSchema(t *testing.T) {
	source, err := os.ReadFile("jobs.go")
	if err != nil {
		t.Fatalf("read jobs.go: %v", err)
	}
	text := string(source)

	if strings.Contains(text, "scan_status") {
		t.Fatal("worker must use file_uploads.status, not removed scan_status column")
	}
	if !strings.Contains(text, "status = 'skipped'") {
		t.Fatal("pending upload cleanup should mark skipped uploads when antivirus is disabled")
	}
	if !strings.Contains(text, "messages") || !strings.Contains(text, "deleted_at <") {
		t.Fatal("message retention must target soft-deleted rows with deleted_at cutoff")
	}
	if !strings.Contains(text, "dm_messages") || !strings.Contains(text, "deleted_at <") {
		t.Fatal("DM retention must target soft-deleted rows with deleted_at cutoff")
	}
}
