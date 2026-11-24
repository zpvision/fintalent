// Package diagnostics keeps a bounded, private error journal. It deliberately
// accepts metadata, not arbitrary messages, request bodies or credentials.
package diagnostics

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxEventBytes = 16 << 10

type Event struct {
	Time          time.Time `json:"time"`
	Source        string    `json:"source"`
	Code          string    `json:"code"`
	Operation     string    `json:"operation,omitempty"`
	Release       string    `json:"release,omitempty"`
	ClientRelease string    `json:"client_release,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	UserID        int64     `json:"user_id,omitempty"`
	CourseID      int64     `json:"course_id,omitempty"`
	EntityID      string    `json:"entity_id,omitempty"`
	Route         string    `json:"route,omitempty"`
	PageRoute     string    `json:"page_route,omitempty"`
	ResourceURL   string    `json:"resource_url,omitempty"`
	ResourceType  string    `json:"resource_type,omitempty"`
	Method        string    `json:"method,omitempty"`
	Status        int       `json:"status,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
	Online        *bool     `json:"online,omitempty"`
	Visibility    string    `json:"visibility,omitempty"`
	Cause         string    `json:"cause,omitempty"`
	Stack         string    `json:"stack,omitempty"`
	Fingerprint   string    `json:"fingerprint"`
}

type Config struct {
	Dir       string
	FileBytes int64
	Files     int
	Retention time.Duration
	Release   string
}

type Store struct {
	mu             sync.Mutex
	cfg            Config
	now            func() time.Time
	dropped        atomic.Uint64
	writeErrors    atomic.Uint64
	lastWriteError time.Time
	// Bound both event rate and disk writes during error storms.
	window     time.Time
	accepted   int
	generation atomic.Uint64
	pending    chan queuedEvent
	stop       chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
}

type queuedEvent struct {
	event      Event
	generation uint64
}

var current atomic.Pointer[Store]

func SetDefault(s *Store) { current.Store(s) }
func Emit(e Event) {
	if s := current.Load(); s != nil {
		s.Submit(e)
	}
}

func (s *Store) Submit(e Event) {
	queued := queuedEvent{event: e, generation: s.generation.Load()}
	select {
	case <-s.stop:
		s.dropped.Add(1)
		return
	default:
	}
	select {
	case s.pending <- queued:
	default:
		s.dropped.Add(1)
	}
}
func (s *Store) Close() { s.closeOnce.Do(func() { close(s.stop); <-s.done }) }
func (s *Store) run() {
	defer close(s.done)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case e := <-s.pending:
			s.record(e.event, e.generation)
		case <-ticker.C:
			if err := s.Cleanup(); err != nil {
				s.writeErrors.Add(1)
			}
		case <-s.stop:
			for {
				select {
				case e := <-s.pending:
					s.record(e.event, e.generation)
				default:
					return
				}
			}
		}
	}
}

func Open(c Config) (*Store, error) {
	if c.FileBytes == 0 {
		c.FileBytes = 5 << 20
	}
	if c.Files == 0 {
		c.Files = 10
	}
	if c.Retention == 0 {
		c.Retention = 14 * 24 * time.Hour
	}
	if c.Dir == "" || c.FileBytes < MaxEventBytes || c.FileBytes > 10<<20 || c.Files < 1 || c.Files > 10 || c.Retention < time.Hour || c.Retention > 30*24*time.Hour {
		return nil, errors.New("invalid diagnostics limits")
	}
	abs, err := filepath.Abs(c.Dir)
	if err != nil {
		return nil, err
	}
	c.Dir = abs
	if err = os.MkdirAll(c.Dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{cfg: c, now: time.Now, pending: make(chan queuedEvent, 256), stop: make(chan struct{}), done: make(chan struct{})}
	if err = s.Cleanup(); err != nil {
		return nil, err
	}
	// Fail early if the journal is not writable; no unbounded stderr fallback.
	f, err := os.OpenFile(s.path(0), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	go s.run()
	return s, nil
}

func (s *Store) path(i int) string {
	return filepath.Join(s.cfg.Dir, fmt.Sprintf("errors-%02d.jsonl", i))
}

func (s *Store) cleanupLocked() error {
	now := s.now()
	// Only touch this journal's fixed filenames, never arbitrary directory entries.
	for i := 0; i < 10; i++ {
		info, err := os.Stat(s.path(i))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if i >= s.cfg.Files || now.Sub(info.ModTime().UTC().Truncate(24*time.Hour)) >= s.cfg.Retention || info.Size() > s.cfg.FileBytes {
			if err = os.Remove(s.path(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Cleanup() error { s.mu.Lock(); defer s.mu.Unlock(); return s.cleanupLocked() }

// Clear removes this journal's files across all periods. Events submitted before
// the clear are discarded even if the background writer has already dequeued them.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation.Add(1)
	for i := 0; i < 10; i++ {
		if err := os.Remove(s.path(i)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	s.window = time.Time{}
	s.accepted = 0
	s.lastWriteError = time.Time{}
	return nil
}

func (s *Store) rotate() error {
	if err := os.Remove(s.path(s.cfg.Files - 1)); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := s.cfg.Files - 2; i >= 0; i-- {
		if err := os.Rename(s.path(i), s.path(i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Store) Record(e Event) {
	s.record(e, s.generation.Load())
}

func (s *Store) record(e Event, generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation.Load() {
		return
	}
	now := s.now()
	if now.Sub(s.window) >= time.Minute || s.window.IsZero() {
		s.window = now
		s.accepted = 0
	}
	if s.accepted >= 300 || !s.lastWriteError.IsZero() && now.Sub(s.lastWriteError) < time.Minute {
		s.dropped.Add(1)
		return
	}
	s.accepted++
	e.Time = now.UTC()
	e.Release = Token(s.cfg.Release, 80)
	e.ClientRelease = Token(e.ClientRelease, 80)
	e.Source = Token(e.Source, 24)
	e.Code = Token(e.Code, 80)
	e.Operation = Token(e.Operation, 100)
	e.RequestID = Token(e.RequestID, 64)
	e.EntityID = Token(e.EntityID, 64)
	e.Method = Token(e.Method, 12)
	e.Route = SafePath(e.Route)
	e.PageRoute = SafePath(e.PageRoute)
	e.ResourceURL = SafeResourceURL(e.ResourceURL)
	e.ResourceType = SafeResourceType(e.ResourceType)
	e.Stack = SafeStack(e.Stack)
	e.Cause = Token(e.Cause, 100)
	if e.Visibility != "visible" && e.Visibility != "hidden" {
		e.Visibility = ""
	}
	if e.DurationMS < 0 {
		e.DurationMS = 0
	}
	if e.DurationMS > 24*60*60*1000 {
		e.DurationMS = 24 * 60 * 60 * 1000
	}
	e.Fingerprint = Fingerprint(e)
	b, err := json.Marshal(e)
	if err != nil || len(b)+1 > MaxEventBytes {
		s.dropped.Add(1)
		return
	}
	err = s.append(now, append(b, '\n'))
	if err != nil {
		s.writeErrors.Add(1)
		s.lastWriteError = now
	}
}

func (s *Store) append(now time.Time, b []byte) error {
	info, err := os.Stat(s.path(0))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Daily rotation ensures even a continuously written file ages out.
	if info != nil && (info.Size()+int64(len(b)) > s.cfg.FileBytes || info.ModTime().UTC().Format("2006-01-02") != now.UTC().Format("2006-01-02")) {
		if err = s.rotate(); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(s.path(0), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func Fingerprint(e Event) string {
	parts := []string{e.Source, e.Code, e.Operation, e.Route, e.Method, e.Cause, e.Stack}
	if e.ResourceURL != "" || e.ResourceType != "" {
		parts = append(parts, e.PageRoute, e.ResourceURL, e.ResourceType)
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(h[:8])
}

type Group struct {
	Category      string    `json:"category"`
	Sample        Event     `json:"sample"`
	Count         int       `json:"count"`
	First         time.Time `json:"first"`
	Last          time.Time `json:"last"`
	AffectedUsers int       `json:"affected_users"`
	MoreUsers     bool      `json:"more_users"`
	users         map[int64]bool
}
type Report struct {
	Since         time.Time `json:"since"`
	Until         time.Time `json:"until"`
	Timezone      string    `json:"timezone"`
	Groups        []Group   `json:"groups"`
	Events        int       `json:"events"`
	Truncated     bool      `json:"truncated"`
	MaxBytes      int64     `json:"max_bytes"`
	StoredBytes   int64     `json:"stored_bytes"`
	RetentionDays int       `json:"retention_days"`
	Dropped       uint64    `json:"dropped_since_start"`
	WriteErrors   uint64    `json:"write_errors_since_start"`
}

func (s *Store) Report(since time.Time) (Report, error) {
	return s.ReportRange(since, time.Now())
}

// ReportRange filters individual events before grouping; both bounds are inclusive.
func (s *Store) ReportRange(since, until time.Time) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Report{Since: since.UTC(), Until: until.UTC(), Timezone: "UTC", Groups: []Group{}, MaxBytes: s.cfg.FileBytes * int64(s.cfg.Files), RetentionDays: int(s.cfg.Retention / (24 * time.Hour)), Dropped: s.dropped.Load(), WriteErrors: s.writeErrors.Load()}
	groups := map[string]*Group{}
	// Read newest files first and keep a recent sample, with bounded groups.
	for i := 0; i < s.cfg.Files; i++ {
		f, err := os.Open(s.path(i))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return r, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return r, err
		}
		r.StoredBytes += info.Size()
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 4096), MaxEventBytes)
		for scan.Scan() {
			var e Event
			if json.Unmarshal(scan.Bytes(), &e) != nil || e.Time.Before(since) || e.Time.After(until) {
				continue
			}
			r.Events++
			g := groups[e.Fingerprint]
			if g == nil {
				if len(groups) >= 500 {
					r.Truncated = true
					continue
				}
				category := "application"
				if e.Source == "frontend" && e.Code == "resource_error" && (strings.HasPrefix(e.ResourceURL, "https://") || strings.HasPrefix(e.ResourceURL, "http://") || e.ResourceURL == "/external/:redacted") {
					category = "external_resource"
				}
				g = &Group{Category: category, Sample: e, First: e.Time, Last: e.Time, users: map[int64]bool{}}
				groups[e.Fingerprint] = g
			}
			g.Count++
			if e.UserID > 0 && !g.users[e.UserID] {
				if len(g.users) < 100 {
					g.users[e.UserID] = true
					g.AffectedUsers++
				} else {
					g.MoreUsers = true
				}
			}
			if e.Time.Before(g.First) {
				g.First = e.Time
			}
			if e.Time.After(g.Last) {
				g.Last = e.Time
				g.Sample = e
			}
		}
		err = scan.Err()
		f.Close()
		if err != nil {
			return r, err
		}
	}
	for _, g := range groups {
		r.Groups = append(r.Groups, *g)
	}
	sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].Last.After(r.Groups[j].Last) })
	return r, nil
}
