package asyncdb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Job struct {
	Name     string
	Key      string
	Group    string
	Payload  any
	Run      func(ctx context.Context) error
	BatchRun func(ctx context.Context, jobs []Job) error
}

type Option func(*Writer)

type Writer struct {
	input         chan Job
	batches       chan []Job
	workers       int
	jobTimeout    time.Duration
	flushInterval time.Duration
	maxBatchSize  int
	logger        *slog.Logger
	startOnce     sync.Once
	stopOnce      sync.Once
	dispatcherWG  sync.WaitGroup
	workerWG      sync.WaitGroup
	lifecycleMu   sync.RWMutex
	closed        atomic.Bool
}

func WithQueueSize(size int) Option {
	return func(w *Writer) {
		if size > 0 {
			w.input = make(chan Job, size)
		}
	}
}

func WithWorkers(workers int) Option {
	return func(w *Writer) {
		if workers > 0 {
			w.workers = workers
		}
	}
}

func WithJobTimeout(timeout time.Duration) Option {
	return func(w *Writer) {
		if timeout > 0 {
			w.jobTimeout = timeout
		}
	}
}

func WithFlushInterval(interval time.Duration) Option {
	return func(w *Writer) {
		if interval > 0 {
			w.flushInterval = interval
		}
	}
}

func WithMaxBatchSize(size int) Option {
	return func(w *Writer) {
		if size > 0 {
			w.maxBatchSize = size
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(w *Writer) {
		if logger != nil {
			w.logger = logger
		}
	}
}

func New(opts ...Option) *Writer {
	w := &Writer{
		input:         make(chan Job, 2048),
		batches:       make(chan []Job, 128),
		workers:       4,
		jobTimeout:    3 * time.Second,
		flushInterval: 20 * time.Millisecond,
		maxBatchSize:  128,
		logger:        slog.Default(),
	}
	for _, opt := range opts {
		opt(w)
	}
	w.start()
	return w
}

func (w *Writer) SetLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	w.logger = logger
}

func (w *Writer) Submit(ctx context.Context, job Job) error {
	if job.Run == nil && job.BatchRun == nil {
		return fmt.Errorf("asyncdb job run and batch run are both nil")
	}
	if job.BatchRun != nil && job.Group == "" {
		job.Group = job.Name
	}
	w.lifecycleMu.RLock()
	defer w.lifecycleMu.RUnlock()
	if w.closed.Load() {
		return fmt.Errorf("asyncdb writer closed")
	}
	select {
	case w.input <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Writer) Stop(ctx context.Context) error {
	w.stopOnce.Do(func() {
		w.lifecycleMu.Lock()
		defer w.lifecycleMu.Unlock()
		w.closed.Store(true)
		close(w.input)
	})

	done := make(chan struct{})
	go func() {
		w.dispatcherWG.Wait()
		w.workerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Writer) start() {
	w.startOnce.Do(func() {
		w.dispatcherWG.Add(1)
		go func() {
			defer w.dispatcherWG.Done()
			defer close(w.batches)

			ticker := time.NewTicker(w.flushInterval)
			defer ticker.Stop()

			normal := make([]Job, 0, w.maxBatchSize)
			dedup := make(map[string]Job)

			flush := func() {
				total := len(normal) + len(dedup)
				if total == 0 {
					return
				}
				batch := make([]Job, 0, total)
				batch = append(batch, normal...)
				for _, job := range dedup {
					batch = append(batch, job)
				}
				normal = normal[:0]
				dedup = make(map[string]Job)
				w.batches <- batch
			}

			for {
				select {
				case job, ok := <-w.input:
					if !ok {
						flush()
						return
					}
					if job.Key != "" {
						dedup[job.Key] = job
					} else {
						normal = append(normal, job)
					}
					if len(normal)+len(dedup) >= w.maxBatchSize {
						flush()
					}
				case <-ticker.C:
					flush()
				}
			}
		}()

		for i := 0; i < w.workers; i++ {
			w.workerWG.Add(1)
			go func() {
				defer w.workerWG.Done()
				for batch := range w.batches {
					grouped := make(map[string][]Job)
					singles := make([]Job, 0, len(batch))

					for _, job := range batch {
						if job.BatchRun != nil {
							grouped[job.Group] = append(grouped[job.Group], job)
							continue
						}
						singles = append(singles, job)
					}

					for group, jobs := range grouped {
						jobCtx, cancel := context.WithTimeout(context.Background(), w.jobTimeout)
						err := jobs[0].BatchRun(jobCtx, jobs)
						cancel()
						if err != nil && w.logger != nil {
							w.logger.Error("[asyncdb] batch job failed", "job", jobs[0].Name, "group", group, "count", len(jobs), "error", err)
						}
					}

					for _, job := range singles {
						jobCtx, cancel := context.WithTimeout(context.Background(), w.jobTimeout)
						err := job.Run(jobCtx)
						cancel()
						if err != nil && w.logger != nil {
							w.logger.Error("[asyncdb] job failed", "job", job.Name, "key", job.Key, "error", err)
						}
					}
				}
			}()
		}
	})
}
