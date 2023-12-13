package rpatterns_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
)

var pMutex = sync.Mutex{}

func TestParallel(t *testing.T) {
	cases := []struct {
		name     string
		n        int
		events   []*reflex.Event
		expexted map[string][]int64
		hash     rpatterns.HashOption
		hashFn   func(*reflex.Event) ([]byte, error)
	}{
		{
			name:   "Hash Event ID",
			n:      4,
			events: fromIDs(0, 1, 2, 3),
			hash:   rpatterns.HashOptionEventID,
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {3},
				"parallel_test_2_of_4": {2},
				"parallel_test_3_of_4": {1},
				"parallel_test_4_of_4": {0},
			},
		},
		{
			name:   "Hash Event Foreign Key",
			n:      4,
			events: fromFIDs(124566, 123412455, 123, 2342, 2304, 140054),
			hash:   rpatterns.HashOptionEventForeignID,
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {2304},
				"parallel_test_2_of_4": {124566, 140054},
				"parallel_test_3_of_4": {123412455, 2342},
				"parallel_test_4_of_4": {123},
			},
		},
		{
			name:   "Hash Event Type",
			n:      4,
			events: fromTypes(1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3),
			hash:   rpatterns.HashOptionEventType,
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {3, 3, 3, 3, 3},
				"parallel_test_2_of_4": {2, 2, 2, 2},
				"parallel_test_3_of_4": {1, 1, 1},
			},
		},
		{
			name:   "Hash Event HashFn (Event ID)",
			n:      4,
			events: fromIDs(0, 1, 2, 3),
			hashFn: func(event *reflex.Event) ([]byte, error) {
				return []byte(event.ID), nil
			},
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {3},
				"parallel_test_2_of_4": {2},
				"parallel_test_3_of_4": {1},
				"parallel_test_4_of_4": {0},
			},
		},
		{
			name:   "Hash Event HashFn (Event Foreign Key)",
			n:      4,
			events: fromFIDs(124566, 123412455, 123, 2342, 2304, 140054),
			hashFn: func(event *reflex.Event) ([]byte, error) {
				return []byte(event.ForeignID), nil
			},
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {2304},
				"parallel_test_2_of_4": {124566, 140054},
				"parallel_test_3_of_4": {123412455, 2342},
				"parallel_test_4_of_4": {123},
			},
		},
		{
			name:   "Hash Event HashFn (Event Type)",
			n:      4,
			events: fromTypes(1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3),
			hashFn: func(event *reflex.Event) ([]byte, error) {
				return []byte(strconv.Itoa(event.Type.ReflexType())), nil
			},
			expexted: map[string][]int64{
				"parallel_test_1_of_4": {3, 3, 3, 3, 3},
				"parallel_test_2_of_4": {2, 2, 2, 2},
				"parallel_test_3_of_4": {1, 1, 1},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			res := make(map[string][]int64)
			cursors := &parallelCursors{
				cursors: make(map[string]string),
				streams: make(map[string]*parallelStream),
				events:  test.events,
			}

			wg.Add(len(test.events))

			fn := func(ctx context.Context, e *reflex.Event) error {
				pMutex.Lock()
				defer pMutex.Unlock()
				defer wg.Done()

				var i int64
				if test.hashFn != nil {
					var err error
					b, err := test.hashFn(e)
					require.NoError(t, err)
					i, err = strconv.ParseInt(string(b), 10, 64)
					require.NoError(t, err)
				} else {
					switch test.hash {
					case rpatterns.HashOptionEventID:
						i = e.IDInt()
					case rpatterns.HashOptionEventForeignID:
						i = e.ForeignIDInt()
					case rpatterns.HashOptionEventType:
						i = int64(e.Type.ReflexType())
					}
				}
				res[ctx.Value("thread").(string)] = append(res[ctx.Value("thread").(string)], i)
				return nil
			}

			getName := func(m int) string {
				return fmt.Sprintf("parallel_test_%d_of_%d", m+1, test.n)
			}

			getCtx := func(m int) context.Context {
				return context.WithValue(context.Background(), "thread", getName(m))
			}

			getConsumer := func(m int) reflex.Consumer {
				return reflex.NewConsumer(getName(m), fn)
			}

			rpatterns.Parallel(getCtx, getConsumer, test.n, cursors.Stream,
				cursors, rpatterns.WithHashOption(test.hash), rpatterns.WithHashFn(test.hashFn))

			wg.Wait()

			require.EqualValues(t, test.expexted, res)
		})
	}
}

func fromIDs(ids ...int) []*reflex.Event {
	var res []*reflex.Event
	for _, i := range ids {
		res = append(res, &reflex.Event{
			ID: strconv.Itoa(i),
		})
	}
	return res
}

func fromTypes(types ...int) []*reflex.Event {
	var res []*reflex.Event
	for _, i := range types {
		res = append(res, &reflex.Event{
			Type: testEventType(i),
		})
	}
	return res
}

func fromFIDs(fids ...int) []*reflex.Event {
	var res []*reflex.Event
	for _, i := range fids {
		res = append(res, &reflex.Event{
			ForeignID: strconv.Itoa(i),
		})
	}
	return res
}

type parallelCursors struct {
	cursors map[string]string
	streams map[string]*parallelStream
	events  []*reflex.Event
}

type parallelStream struct {
	events []*reflex.Event
	index  int
}

func (p *parallelStream) Recv() (*reflex.Event, error) {
	if len(p.events) <= p.index {
		return nil, errEvents
	}
	e := p.events[p.index]
	p.index++
	return e, nil
}

func (p *parallelCursors) GetCursor(ctx context.Context, consumerName string) (string, error) {
	pMutex.Lock()
	defer pMutex.Unlock()
	return p.cursors[consumerName], nil
}

func (p *parallelCursors) SetCursor(ctx context.Context, consumerName string, cursor string) error {
	pMutex.Lock()
	defer pMutex.Unlock()
	p.cursors[consumerName] = cursor
	return nil
}

func (*parallelCursors) Flush(ctx context.Context) error {
	return nil
}

func (p *parallelCursors) Stream(ctx context.Context, after string, opts ...reflex.StreamOption) (reflex.StreamClient, error) {
	pMutex.Lock()
	s, ok := p.streams[ctx.Value("thread").(string)]
	if ok {
		pMutex.Unlock()
		return s, nil
	}
	p.streams[ctx.Value("thread").(string)] = &parallelStream{events: p.events, index: 0}
	pMutex.Unlock()

	return p.Stream(ctx, after, opts...)
}
