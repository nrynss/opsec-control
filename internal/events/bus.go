package events

import (
	"sync"

	"github.com/nrynss/opsec-control/internal/contracts"
)

const defaultBuffer = 64

// Bus is an in-memory implementation of contracts.EventBus. It owns no domain
// state; it only distributes immutable events to active subscribers.
type Bus struct {
	mu          sync.Mutex
	nextID      uint64
	buffer      int
	subscribers map[uint64]*subscriber
}

var _ contracts.EventBus = (*Bus)(nil)

// New returns an in-memory event bus. buffer controls each subscriber's output
// channel capacity; values below 1 use the package default.
func New(buffer int) *Bus {
	if buffer < 1 {
		buffer = defaultBuffer
	}
	return &Bus{
		buffer:      buffer,
		subscribers: make(map[uint64]*subscriber),
	}
}

// Publish distributes event to every subscriber active at the start of the
// call. Each subscriber has an independent FIFO, so a slow consumer does not
// block delivery to other subscribers.
func (b *Bus) Publish(event contracts.Event) {
	b.mu.Lock()
	subs := make([]*subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	for _, sub := range subs {
		sub.publish(event)
	}
}

// Subscribe registers a new subscriber and returns its receive stream plus an
// idempotent cancel function. Cancel stops future deliveries and closes the
// receive channel.
func (b *Bus) Subscribe() (<-chan contracts.Event, func()) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	sub := newSubscriber(b.buffer)
	b.subscribers[id] = sub
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if current, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			current.cancel()
		}
		b.mu.Unlock()
	}

	return sub.events(), cancel
}

type subscriber struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  []contracts.Event
	out    chan contracts.Event
	done   chan struct{}
	closed bool
	once   sync.Once
}

func newSubscriber(buffer int) *subscriber {
	sub := &subscriber{
		out:  make(chan contracts.Event, buffer),
		done: make(chan struct{}),
	}
	sub.cond = sync.NewCond(&sub.mu)
	go sub.run()
	return sub
}

func (s *subscriber) events() <-chan contracts.Event {
	return s.out
}

func (s *subscriber) publish(event contracts.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.queue = append(s.queue, event)
	s.cond.Signal()
}

func (s *subscriber) cancel() {
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.closed = true
		s.queue = nil
		s.cond.Broadcast()
		s.mu.Unlock()
	})
}

func (s *subscriber) run() {
	defer close(s.out)
	for {
		event, ok := s.next()
		if !ok {
			return
		}
		select {
		case s.out <- event:
		case <-s.done:
			return
		}
	}
}

func (s *subscriber) next() (contracts.Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.queue) == 0 && !s.closed {
		s.cond.Wait()
	}
	if len(s.queue) == 0 {
		return contracts.Event{}, false
	}
	event := s.queue[0]
	copy(s.queue, s.queue[1:])
	s.queue = s.queue[:len(s.queue)-1]
	return event, true
}
