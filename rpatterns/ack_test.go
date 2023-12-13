package rpatterns_test

import (
	"context"
	"log"
	"reflect"
	"testing"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/stretchr/testify/assert"
)

func TestAck(t *testing.T) {
	cases := []struct {
		name         string
		inEvents     []int
		acks         []string
		opts         []reflex.StreamOption
		consumerErr  string
		consumerErr2 string
	}{
		{
			name:     "no acks",
			inEvents: []int{1, 2, 3, 4},
		}, {
			name:     "all acks",
			inEvents: []int{1, 2, 3, 4},
			acks:     []string{"1", "2", "3", "4"},
		}, {
			name:     "ack 2",
			inEvents: []int{1, 2, 3, 4},
			acks:     []string{"2"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			events := ItoEList(test.inEvents...)
			b := new(bootstrapMock)
			b.gets = []string{""}
			b.events = events

			var results []*reflex.Event
			consumer := rpatterns.NewAckConsumer("test", b,
				func(ctx context.Context, e *rpatterns.AckEvent) error {
					results = append(results, &e.Event)
					for _, id := range test.acks {
						if id == e.ID {
							assert.NoError(t, e.Ack(ctx))
						}
					}
					return nil
				})

			req := rpatterns.NewAckSpec(b.Stream, consumer)
			err := reflex.Run(context.Background(), req)
			assert.EqualError(t, err, "recv error: no more events")

			assert.EqualValues(t, test.acks, b.sets)
			assert.Equal(t, len(test.acks)+1, b.flushes)
			assert.True(t, reflect.DeepEqual(results, events),
				"Mismatching result", results, events)
		})
	}
}

func TestAckExample(t *testing.T) {
	// Init events and mocks
	events := ItoEList(1, 2, 3, 4, 5, 6)
	b := new(bootstrapMock)
	b.gets = []string{""}
	b.events = events

	batcher := makeBatcher(b)
	req := rpatterns.NewAckSpec(b.Stream, batcher)
	err := reflex.Run(context.Background(), req)
	assert.EqualError(t, err, "recv error: no more events")
	assert.EqualValues(t, []string{"2", "4", "6"}, b.sets)
}

// makeBatcher returns a simple batch processor.
func makeBatcher(cstore reflex.CursorStore) *rpatterns.AckConsumer {
	var (
		batch     []rpatterns.AckEvent
		batchSize = 2
	)

	f := func(ctx context.Context, e *rpatterns.AckEvent) error {
		batch = append(batch, *e)

		if len(batch) >= batchSize {
			log.Printf("batch processed, len=%d", len(batch))

			// Ack last event in batch
			if err := batch[len(batch)-1].Ack(ctx); err != nil {
				return err
			}
			batch = nil
		}
		return nil
	}

	return rpatterns.NewAckConsumer("batcher", cstore, f)
}
