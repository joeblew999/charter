// Package follow is the one way a client receives a live, ordered, gap-free feed
// (.plans/realtime.md, rule 3): the Go port of api/src/follow.ts, with the same tests. Both transports
// (SSE and the WebSocket) are thin adapters over Follow.
//
// The log (D1) is the source of truth and the item id is the only position. The live source (the hub
// Durable Object) is only a wake-up signal and may drop, restart, or deliver out of order:
//   - subscribe first, then catch up from the log (id > after), then go live;
//   - a live item is emitted directly only when it is the next id; otherwise the log is read, which
//     fills gaps and restores order (a single SQLite writer makes a visible id imply all lower ones);
//   - on a hub error, resubscribe and catch up from the last id emitted;
//   - when idle for Recheck, catch up from the log anyway, so a hub that closes silently costs at
//     most that much delay, never a lost item.
//
// Follow returns an error only after MaxFailures consecutive failed subscribes, when the log fails,
// or when emit does, so the adapter can end the stream explicitly (rule 5); a cancelled context ends
// it quietly.
package follow

import (
	"context"
	"sync"
	"time"
)

// Item is anything with a position in the log.
type Item interface {
	Position() int64
}

// Source is the log plus the live signal.
type Source[T Item] interface {
	// Subscribe to live items; onError is called when the subscription is broken. Neither callback
	// may block. It returns unsubscribe.
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)
	// Since returns items with id > after, ascending, at most limit.
	Since(ctx context.Context, after int64, limit int) ([]T, error)
	// Latest is the newest id in the log (0 when empty): where a follower without After starts.
	Latest(ctx context.Context) (int64, error)
}

// Options tune Follow; the zero value is the production setting.
type Options struct {
	// After is the resume position: emit items with id > After. Nil: only items newer than now.
	After *int64
	// Recheck: read the log when nothing has arrived for this long (default 30 s).
	Recheck time.Duration
	// Retry: wait between resubscribes after a hub failure (default 1 s).
	Retry time.Duration
	// MaxFailures: consecutive failed subscribes before giving up (default 5).
	MaxFailures int
	// PageSize for log reads (default 100).
	PageSize int
	// OnBroken is called when the live subscription breaks, before resubscribing: for logs.
	OnBroken func(error)
}

// Follow emits the feed until ctx is cancelled.
func Follow[T Item](ctx context.Context, src Source[T], opt Options, emit func(T) error) error {
	if opt.Recheck <= 0 {
		opt.Recheck = 30 * time.Second
	}
	if opt.Retry <= 0 {
		opt.Retry = time.Second
	}
	if opt.MaxFailures <= 0 {
		opt.MaxFailures = 5
	}
	if opt.PageSize <= 0 {
		opt.PageSize = 100
	}

	var last int64
	if opt.After != nil {
		last = *opt.After
	} else {
		latest, err := src.Latest(ctx)
		if err != nil {
			return quiet(ctx, err)
		}
		last = latest
	}

	// Everything the log has after last, in pages.
	catchUp := func() error {
		for {
			page, err := src.Since(ctx, last, opt.PageSize)
			if err != nil {
				return err
			}
			for _, item := range page {
				if item.Position() > last {
					last = item.Position()
					if err := emit(item); err != nil {
						return err
					}
				}
			}
			if len(page) < opt.PageSize {
				return nil
			}
		}
	}

	failures := 0
	for ctx.Err() == nil {
		live := &queue[T]{wake: make(chan struct{}, 1)}
		unsubscribe, err := src.Subscribe(live.push, live.fail)
		if err != nil {
			failures++
			if failures >= opt.MaxFailures {
				return err
			}
			sleep(ctx, opt.Retry)
			continue
		}
		broken, err := func() (error, error) {
			defer unsubscribe()
			if err := catchUp(); err != nil {
				return nil, err
			}
			failures = 0
			for ctx.Err() == nil {
				item, ok, broken := live.pop()
				if broken != nil {
					return broken, nil
				}
				if ok {
					switch id := item.Position(); {
					case id == last+1:
						last = id
						if err := emit(item); err != nil {
							return nil, err
						}
					case id > last: // a gap or out of order: the log decides
						if err := catchUp(); err != nil {
							return nil, err
						}
					}
					continue
				}
				idle := time.NewTimer(opt.Recheck)
				select {
				case <-live.wake:
					idle.Stop()
				case <-ctx.Done():
					idle.Stop()
				case <-idle.C: // idle: covers a hub that closed silently
					if err := catchUp(); err != nil {
						return nil, err
					}
				}
			}
			return nil, nil
		}()
		if err != nil {
			return quiet(ctx, err)
		}
		if broken != nil && ctx.Err() == nil {
			if opt.OnBroken != nil {
				opt.OnBroken(broken)
			}
			sleep(ctx, opt.Retry)
		}
	}
	return nil
}

// quiet hides the errors a cancelled context causes: an abort is not a failure.
func quiet(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// queue is what a subscription delivers into: push and fail never block, wake nudges the loop.
type queue[T Item] struct {
	mu     sync.Mutex
	items  []T
	broken error
	wake   chan struct{}
}

func (q *queue[T]) push(item T) {
	q.mu.Lock()
	q.items = append(q.items, item)
	q.mu.Unlock()
	q.nudge()
}

func (q *queue[T]) fail(err error) {
	if err == nil {
		err = errBroken
	}
	q.mu.Lock()
	if q.broken == nil {
		q.broken = err
	}
	q.mu.Unlock()
	q.nudge()
}

func (q *queue[T]) nudge() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *queue[T]) pop() (item T, ok bool, broken error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.broken != nil {
		return item, false, q.broken
	}
	if len(q.items) == 0 {
		return item, false, nil
	}
	item, q.items = q.items[0], q.items[1:]
	return item, true, nil
}

type brokenError struct{}

func (brokenError) Error() string { return "subscription broken" }

var errBroken error = brokenError{}
