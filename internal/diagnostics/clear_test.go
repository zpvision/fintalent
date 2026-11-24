package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClearJournalAndQueuedEvents(t *testing.T) {
	s := testStore(t, 3)
	for i := 0; i < 300; i++ {
		s.Record(Event{Source: "backend", Code: "old", Stack: string(make([]byte, 1000))})
	}
	// Fill the queue while the writer cannot acquire the disk lock.
	s.mu.Lock()
	for i := 0; i < 100; i++ {
		s.Submit(Event{Source: "frontend", Code: "queued_old"})
	}
	s.mu.Unlock()
	oldGeneration := s.generation.Load()
	other := filepath.Join(s.cfg.Dir, "keep.jsonl")
	if err := os.WriteFile(other, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	// Simulate an old event already dequeued by the background writer.
	s.record(Event{Source: "worker", Code: "dequeued_old"}, oldGeneration)
	r, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil || r.Events != 0 || r.StoredBytes != 0 {
		t.Fatalf("clear left old events: %+v %v", r, err)
	}
	if b, err := os.ReadFile(other); err != nil || string(b) != "unrelated" {
		t.Fatal("unrelated file changed")
	}
	s.Submit(Event{Source: "backend", Code: "new"})
	s.Close() // Flush both the old queue and new event.
	r, err = s.Report(time.Now().Add(-time.Hour))
	if err != nil || r.Events != 1 || r.Groups[0].Sample.Code != "new" {
		t.Fatalf("new events lost or old queue resurrected: %+v %v", r, err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal("clearing an empty journal must succeed", err)
	}
	reopened, err := Open(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	r, err = reopened.Report(time.Now().Add(-time.Hour))
	if err != nil || r.Events != 0 {
		t.Fatal("cleared events returned after restart")
	}
}

func TestClearReportsFileErrors(t *testing.T) {
	s := testStore(t, 3)
	if err := os.Remove(s.path(0)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.path(0), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.path(0), "keep"), []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err == nil {
		t.Fatal("file deletion error was hidden")
	}
}
