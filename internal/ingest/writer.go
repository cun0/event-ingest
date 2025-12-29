package ingest

import (
	"context"
	"errors"
	"time"

	"github.com/cun0/insider-case/internal/domain"
	"github.com/cun0/insider-case/internal/jsonlog"
)

var ErrStopped = errors.New("ingest writer stopped")

type batchRepo interface {
	InsertBatch(ctx context.Context, events []domain.Event) (map[string]struct{}, error)
}

type request struct {
	ev   domain.Event
	resp chan response
}

type response struct {
	res Result
	err error
}

type Config struct {
	BatchWindow time.Duration
	MaxBatch    int
	QueueSize   int
}

type SingleWriter struct {
	repo   batchRepo
	cfg    Config
	logger *jsonlog.Logger

	in     chan request
	stopCh chan struct{}
	doneCh chan struct{}
}

func NewSingleWriter(repo batchRepo, cfg Config, logger *jsonlog.Logger) *SingleWriter {
	if cfg.BatchWindow <= 0 {
		cfg.BatchWindow = 2 * time.Millisecond
	}
	if cfg.MaxBatch <= 0 {
		cfg.MaxBatch = 800
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 50_000
	}

	return &SingleWriter{
		repo:   repo,
		cfg:    cfg,
		logger: logger,
		in:     make(chan request, cfg.QueueSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (w *SingleWriter) Start() error {
	go w.loop()
	return nil
}

func (w *SingleWriter) Stop(ctx context.Context) error {
	// Signal stop (idempotent close pattern).
	select {
	case <-w.stopCh:
		// already stopping
	default:
		close(w.stopCh)
	}

	select {
	case <-w.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *SingleWriter) Submit(ctx context.Context, e domain.Event) (Result, error) {
	// Fast reject if stopping.
	select {
	case <-w.stopCh:
		return Result{}, ErrStopped
	default:
	}

	req := request{
		ev:   e,
		resp: make(chan response, 1), // must be buffered to avoid writer blocking
	}

	// orr exit on ctx / stop
	select {
	case w.in <- req:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-w.stopCh:
		return Result{}, ErrStopped
	}

	// Wait for result (or exit on ctx).
	select {
	case out := <-req.resp:
		return out.res, out.err
	case <-ctx.Done():
		// The writer may still commit later; i just stop waiting.
		return Result{}, ctx.Err()
	}
}

func (w *SingleWriter) loop() {
	defer close(w.doneCh)

	batch := make([]request, 0, w.cfg.MaxBatch)

	timer := time.NewTimer(w.cfg.BatchWindow)
	timer.Stop()
	var timerC <-chan time.Time // nil until we arm the timer

	stopping := false

	for {
		// If stopping: wait for the channel to become quiet, then flush and exit.
		if stopping {
			quiet := time.NewTimer(w.cfg.BatchWindow)
			defer quiet.Stop()

			for {
				select {
				case req := <-w.in:
					batch = append(batch, req)
					if len(batch) >= w.cfg.MaxBatch {
						w.flush(batch)
						batch = batch[:0]
					}
					// reset quiet timer
					if !quiet.Stop() {
						select {
						case <-quiet.C:
						default:
						}
					}
					quiet.Reset(w.cfg.BatchWindow)

				case <-quiet.C:
					if len(batch) > 0 {
						w.flush(batch)
					}
					return
				}
			}
		}

		select {
		case <-w.stopCh:
			// Start graceful stop: keep draining for a short window, then exit.
			stopping = true
			// disarm timer
			if timerC != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timerC = nil
			}

		case req := <-w.in:
			batch = append(batch, req)

			// Arm timer when batch starts.
			if timerC == nil {
				timer.Reset(w.cfg.BatchWindow)
				timerC = timer.C
			}

			// Flush if we hit max batch.
			if len(batch) >= w.cfg.MaxBatch {
				if timerC != nil {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timerC = nil
				}
				w.flush(batch)
				batch = batch[:0]
			}

		case <-timerC:
			timerC = nil
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (w *SingleWriter) flush(batch []request) {
	if w.logger != nil {
		w.logger.PrintInfo("flush batch", map[string]string{
			"batch_size": itoa(len(batch)),
		})
	}
	events := make([]domain.Event, 0, len(batch))
	for _, r := range batch {
		events = append(events, r.ev)
	}

	// bounded context to avoid hanging forever on DB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	insertedKeys, err := w.repo.InsertBatch(ctx, events)
	if err != nil && w.logger != nil {
		w.logger.PrintError(err, map[string]string{
			"component":  "ingest_writer",
			"batch_size": itoa(len(batch)),
		})
	}

	for _, r := range batch {
		var out response
		if err != nil {
			out.err = err
		} else {
			if _, ok := insertedKeys[r.ev.DedupKey]; ok {
				out.res = Result{Status: StatusInserted}
			} else {
				out.res = Result{Status: StatusDuplicate}
			}
		}

		// TODO: Add a timeout for the response
		// Never block here. Client may have already timed out.
		select {
		case r.resp <- out:
		default:
		}
	}
}

// small helper to avoid fmt in a hot-ish path
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(buf[i:])
}
