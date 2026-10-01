package follow

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

type item int64

func (i item) Position() int64 { return int64(i) }

// fake is a log plus a hub: add appends to the log and delivers live (or not, to simulate loss),
// publish delivers live only, breakHub fails every current subscription, failSubscribes makes the
// next n subscribes fail.
type fake struct {
	mu         sync.Mutex
	log        []item
	subs       map[int]*sub
	next       int
	failNext   int
	subscribes int
	beforeRead func() // runs once, inside the next Since
}

type sub struct {
	listener func(item)
	onError  func(error)
}

func newFake(initial int) *fake {
	f := &fake{subs: map[int]*sub{}}
	for i := 1; i <= initial; i++ {
		f.log = append(f.log, item(i))
	}
	return f
}

func (f *fake) Subscribe(listener func(item), onError func(error)) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribes++
	if f.failNext > 0 {
		f.failNext--
		return nil, errors.New("hub unavailable")
	}
	n := f.next
	f.next++
	f.subs[n] = &sub{listener, onError}
	return func() { f.mu.Lock(); delete(f.subs, n); f.mu.Unlock() }, nil
}

func (f *fake) Since(_ context.Context, after int64, limit int) ([]item, error) {
	f.mu.Lock()
	hook := f.beforeRead
	f.beforeRead = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []item
	for _, it := range f.log {
		if int64(it) > after && len(out) < limit {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *fake) Latest(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.log)), nil
}

func (f *fake) add(live bool) item {
	f.mu.Lock()
	it := item(len(f.log) + 1)
	f.log = append(f.log, it)
	f.mu.Unlock()
	if live {
		f.publish(it)
	}
	return it
}

func (f *fake) publish(it item) {
	f.mu.Lock()
	var current []*sub
	for _, s := range f.subs {
		current = append(current, s)
	}
	f.mu.Unlock()
	for _, s := range current {
		s.listener(it)
	}
}

func (f *fake) breakHub() {
	f.mu.Lock()
	current := f.subs
	f.subs = map[int]*sub{}
	f.mu.Unlock()
	for _, s := range current {
		s.onError(errors.New("closed 1006"))
	}
}

func (f *fake) count() (subscribers, subscribes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subs), f.subscribes
}

var fast = Options{Recheck: 50 * time.Millisecond, Retry: 5 * time.Millisecond}

func after(n int64, opt Options) Options { opt.After = &n; return opt }

// take runs a follower until n items arrive (or 2 s pass) and returns their ids and Follow's error.
func take(f *fake, opt Options, n int) func() ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	var got []int64
	done := make(chan error, 1)
	go func() {
		done <- Follow[item](ctx, f, opt, func(it item) error {
			got = append(got, int64(it))
			if len(got) >= n {
				cancel()
			}
			return nil
		})
	}()
	return func() ([]int64, error) { defer cancel(); err := <-done; return got, err }
}

func tick(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func expect(t *testing.T, wait func() ([]int64, error), want ...int64) {
	t.Helper()
	got, err := wait()
	if err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCatchesUpThenGoesLiveInOrder(t *testing.T) {
	f := newFake(5)
	got := take(f, after(2, fast), 5)
	tick(5)
	f.add(true)
	f.add(true)
	expect(t, got, 3, 4, 5, 6, 7)
}

func TestWithoutAfterStartsFromNow(t *testing.T) {
	f := newFake(3)
	got := take(f, fast, 2)
	tick(5)
	f.add(true)
	f.add(true)
	expect(t, got, 4, 5)
}

func TestItemCreatedBetweenSubscribeAndCatchUpIsEmittedOnce(t *testing.T) {
	f := newFake(0)
	f.beforeRead = func() { f.add(true) }
	got := take(f, after(0, fast), 2)
	tick(5)
	f.add(true)
	expect(t, got, 1, 2)
}

func TestHubDropIsInvisible(t *testing.T) {
	f := newFake(0)
	got := take(f, after(0, fast), 4)
	tick(5)
	f.add(true)
	tick(5)
	f.breakHub()
	f.add(false) // created while no one is subscribed
	f.add(false)
	tick(30)
	f.add(true)
	expect(t, got, 1, 2, 3, 4)
	if _, subscribes := f.count(); subscribes != 2 {
		t.Fatalf("subscribes = %d, want 2", subscribes)
	}
}

func TestSilentHubCostsAtMostRecheck(t *testing.T) {
	f := newFake(0)
	got := take(f, after(0, fast), 3)
	tick(5)
	f.add(false) // never delivered live
	f.add(false)
	f.add(false)
	expect(t, got, 1, 2, 3)
}

func TestOutOfOrderLiveDeliveryIsReorderedFromTheLog(t *testing.T) {
	f := newFake(0)
	got := take(f, after(0, fast), 3)
	tick(5)
	a, b, c := f.add(false), f.add(false), f.add(false)
	f.publish(c)
	f.publish(a)
	f.publish(b)
	expect(t, got, 1, 2, 3)
}

func TestGivesUpAfterMaxFailures(t *testing.T) {
	f := newFake(0)
	f.failNext = 10
	opt := after(0, fast)
	opt.MaxFailures = 3
	_, err := take(f, opt, 1)()
	if err == nil || err.Error() != "hub unavailable" {
		t.Fatalf("err = %v, want hub unavailable", err)
	}
	if _, subscribes := f.count(); subscribes != 3 {
		t.Fatalf("subscribes = %d, want 3", subscribes)
	}
}

func TestRecoversWhenTheHubComesBack(t *testing.T) {
	f := newFake(0)
	f.failNext = 2
	got := take(f, after(0, fast), 1)
	tick(40)
	f.add(true)
	expect(t, got, 1)
}

func TestCancelEndsQuietlyAndUnsubscribes(t *testing.T) {
	f := newFake(0)
	ctx, cancel := context.WithCancel(context.Background())
	var got []int64
	done := make(chan error, 1)
	go func() {
		done <- Follow[item](ctx, f, after(0, fast), func(it item) error { got = append(got, int64(it)); return nil })
	}()
	tick(5)
	f.add(true)
	tick(5)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if !slices.Equal(got, []int64{1}) {
		t.Fatalf("got %v, want [1]", got)
	}
	if subscribers, _ := f.count(); subscribers != 0 {
		t.Fatalf("subscribers = %d, want 0", subscribers)
	}
}

func TestPagesThroughALongCatchUp(t *testing.T) {
	f := newFake(250)
	opt := after(0, fast)
	opt.PageSize = 100
	got, err := take(f, opt, 250)()
	if err != nil || len(got) != 250 {
		t.Fatalf("got %d items, err %v; want 250", len(got), err)
	}
}

func TestEmitErrorEndsTheFeed(t *testing.T) {
	f := newFake(3)
	gone := errors.New("client gone")
	err := Follow[item](context.Background(), f, after(0, fast), func(item) error { return gone })
	if !errors.Is(err, gone) {
		t.Fatalf("err = %v, want client gone", err)
	}
	if subscribers, _ := f.count(); subscribers != 0 {
		t.Fatalf("subscribers = %d, want 0", subscribers)
	}
}
