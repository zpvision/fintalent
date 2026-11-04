// Command fintalent-storage-check verifies configured storage using only its own
// randomly named object below integration-tests/. No database access is needed.
package main

import (
	"FinTalent/internal/objectstorage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func main() {
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, "storage_check_failed")
		os.Exit(1)
	}
}

func check() error {
	c, err := objectstorage.ConfigFrom(os.Getenv, true)
	if err != nil {
		return objectstorage.ErrInvalid
	}
	s := objectstorage.NewS3(c)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = s.Check(ctx); err != nil {
		return err
	}
	k, err := objectstorage.UserKey(1, "files")
	if err != nil {
		return err
	}
	key := "integration-tests/" + strings.TrimPrefix(k, "users/1/files/") + "/deployment-check.txt"
	defer func() {
		clean, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_ = s.Delete(clean, key)
	}()
	const content = "FinTalent isolated deployment storage check\n"
	if err = s.Put(ctx, key, strings.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		return err
	}
	info, err := s.Stat(ctx, key)
	if err != nil || info.Size != int64(len(content)) {
		return objectstorage.ErrUnavailable
	}
	body, _, err := s.Open(ctx, key, "")
	if err != nil {
		return err
	}
	b, readErr := io.ReadAll(io.LimitReader(body, int64(len(content)+1)))
	body.Close()
	if readErr != nil || string(b) != content {
		return objectstorage.ErrUnavailable
	}
	if err = s.Delete(ctx, key); err != nil {
		return err
	}
	if _, err = s.Stat(ctx, key); err != objectstorage.ErrNotFound {
		return objectstorage.ErrUnavailable
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"storage_check": "passed", "upload": true, "download": true, "delete": true, "test_key": key})
}
