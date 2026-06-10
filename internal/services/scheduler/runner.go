// Package scheduler runs periodic maintenance jobs: retention sweeps,
// queue-table upkeep, vector reindexing and memory compaction.
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

type Job struct {
	Name     string
	Interval time.Duration
	Run      func(context.Context) error
}

type Runner struct {
	jobs []Job
}

func NewRunner(jobs ...Job) *Runner {
	return &Runner{jobs: jobs}
}

// Start runs every job on its own ticker until the context is canceled.
// Job errors are logged and the job keeps its schedule.
func (r *Runner) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, job := range r.jobs {
		if job.Interval <= 0 || job.Run == nil {
			continue
		}
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			ticker := time.NewTicker(job.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := job.Run(ctx); err != nil && ctx.Err() == nil {
						log.Printf("scheduler job %s failed: %v", job.Name, err)
					}
				}
			}
		}(job)
	}
	wg.Wait()
	return nil
}
