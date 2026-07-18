package bot

import (
	"context"
	"fmt"
	"io"
	"time"
)

type BoardAdapter interface {
	PollSince(ctx context.Context, cp Checkpoint) ([]BoardItem, Checkpoint, error)
}

type Poller struct {
	adapter    BoardAdapter
	store      *StateStore
	log        io.Writer
	dispatcher *Dispatcher
}

func NewPoller(adapter BoardAdapter, store *StateStore, log io.Writer) *Poller {
	if log == nil {
		log = io.Discard
	}
	return &Poller{
		adapter: adapter,
		store:   store,
		log:     log,
	}
}

func (p *Poller) SetDispatcher(dispatcher *Dispatcher) {
	p.dispatcher = dispatcher
}

func (p *Poller) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if _, err := p.RunOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Poller) RunOnce(ctx context.Context) ([]BoardItem, error) {
	state, err := p.store.Load()
	if err != nil {
		return nil, err
	}
	items, next, err := p.adapter.PollSince(ctx, state.Checkpoint)
	if err != nil {
		return nil, err
	}
	next.LastCheckedAt = time.Now().UTC()
	for _, item := range items {
		fmt.Fprintf(p.log, "new board item: id=%s channel=%s author=%s\n", item.ID, item.ChannelID, item.AuthorID)
	}
	state.Checkpoint = next
	if p.dispatcher != nil {
		state, err = p.dispatcher.DispatchNewItems(ctx, state, items)
		if err != nil {
			return nil, err
		}
	}
	if err := p.store.Save(state); err != nil {
		return nil, err
	}
	return items, nil
}
