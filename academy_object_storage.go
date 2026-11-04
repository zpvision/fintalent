package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/objectstorage"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

var academyObjects academy.StorageOptions
var academyObjectsOnce sync.Once
var objectStorageCSPSource string

func academyStorageOptions() academy.StorageOptions {
	academyObjectsOnce.Do(func() {
		c, e := objectstorage.ConfigFrom(os.Getenv, strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production"))
		if e != nil {
			panic(e.Error())
		}
		academyObjects = academy.StorageOptions{Driver: c.Driver, TTL: c.TTL, VideoMax: c.VideoMax}
		if c.Driver == "s3" {
			s := objectstorage.NewS3(c)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if e = s.Check(ctx); e != nil {
				panic("Academy S3 is unavailable or bucket access denied")
			}
			academyObjects.Primary = s
			academyObjects.Multipart = s
			if origins := objectstorage.BrowserOrigins(c); len(origins) > 0 {
				objectStorageCSPSource = " " + strings.Join(origins, " ")
			}
		}
	})
	return academyObjects
}
func startAcademyStorageWorker(ctx context.Context, wg *sync.WaitGroup) {
	h := academy.New(db, nil, nil, academyStorageRoot())
	options := academyStorageOptions()
	h.SetStorage(options)
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		lastCleanup := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				started := time.Now()
				job, cancel := context.WithTimeout(ctx, 5*time.Minute)
				e := h.ProcessUploads(job)
				if time.Since(lastCleanup) > time.Minute {
					if er := h.CleanupStorage(job); e == nil {
						e = er
					}
					lastCleanup = time.Now()
				}
				cancel()
				if e != nil && ctx.Err() == nil {
					slog.Error("academy_storage_operation_failed", "operation", "finalize_or_cleanup", "driver", options.Driver, "duration_ms", time.Since(started).Milliseconds(), "error_code", "operation_failed")
				}
			}
		}
	}()
}
