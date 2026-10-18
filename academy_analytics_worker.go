package main

import (
	"FinTalent/internal/academy"
	"context"
	"log"
	"sync"
	"time"
)

func startAcademyAnalyticsWorker(ctx context.Context, wg *sync.WaitGroup) {
	root := academyStorageRoot()
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				job, cancel := context.WithTimeout(ctx, 5*time.Minute)
				err := academy.ProcessAnalyticsReports(job, db, root)
				cancel()
				if err != nil && ctx.Err() == nil {
					log.Print("academy report processing failed")
				}
			}
		}
	}()
}
