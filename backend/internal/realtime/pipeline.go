package realtime

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// UserEvent is one event for one user in a multi-user publish.
type UserEvent struct {
	UserID uuid.UUID
	Data   any
}

// pipelineChunk bounds how many commands one Redis round trip carries.
const pipelineChunk = 500

// PipelineBroker is implemented by brokers that can publish many durable
// events in few round trips (broadcast fan-out, notification batches).
type PipelineBroker interface {
	// PublishPipelined publishes payloads[i] to users[i] as durable events and
	// returns the indexes that failed.
	PublishPipelined(ctx context.Context, users []uuid.UUID, payloads [][]byte) (failed []int, err error)
}

// PublishPipelined implements PipelineBroker: the publish script is invoked
// by SHA for a whole chunk in one pipeline; anything that failed (for
// example NOSCRIPT after a Redis restart) is retried one by one.
func (b *RedisBroker) PublishPipelined(ctx context.Context, users []uuid.UUID, payloads [][]byte) ([]int, error) {
	if len(users) != len(payloads) {
		return nil, errors.New("realtime: users and payloads differ in length")
	}
	if len(users) == 0 {
		return nil, nil
	}
	if err := publishScript.Load(ctx, b.client).Err(); err != nil {
		return b.publishEach(ctx, users, payloads, allIndexes(len(users))), nil
	}
	ttl := b.streamTTL.Milliseconds()
	var failed []int
	for start := 0; start < len(users); start += pipelineChunk {
		end := min(start+pipelineChunk, len(users))
		pipe := b.client.Pipeline()
		cmds := make([]*redis.Cmd, 0, end-start)
		for i := start; i < end; i++ {
			cmds = append(cmds, pipe.EvalSha(ctx, publishScript.Hash(), []string{b.streamKey(users[i]), b.userChan(users[i])},
				b.maxLen, string(payloads[i]), ttl))
		}
		_, _ = pipe.Exec(ctx) // per-command errors are inspected below
		var retry []int
		for k, c := range cmds {
			if c.Err() != nil {
				retry = append(retry, start+k)
			}
		}
		failed = append(failed, b.publishEach(ctx, users, payloads, retry)...)
	}
	return failed, nil
}

func (b *RedisBroker) publishEach(ctx context.Context, users []uuid.UUID, payloads [][]byte, idx []int) []int {
	var failed []int
	for _, i := range idx {
		if _, err := b.Publish(ctx, users[i], payloads[i]); err != nil {
			failed = append(failed, i)
		}
	}
	return failed
}

func allIndexes(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// PublishPipelined implements PipelineBroker.
func (b *MemoryBroker) PublishPipelined(ctx context.Context, users []uuid.UUID, payloads [][]byte) ([]int, error) {
	if len(users) != len(payloads) {
		return nil, errors.New("realtime: users and payloads differ in length")
	}
	for i := range users {
		if _, err := b.Publish(ctx, users[i], payloads[i]); err != nil {
			return []int{i}, err
		}
	}
	return nil, nil
}

// PublishMany sends a durable event of the same type with per-user data to
// many users, pipelined when the broker supports it. It returns the indexes
// (into events) that could not be published, so callers can retry only those.
func (p *Publisher) PublishMany(ctx context.Context, eventType string, events []UserEvent) ([]int, error) {
	users := make([]uuid.UUID, 0, len(events))
	payloads := make([][]byte, 0, len(events))
	pos := make([]int, 0, len(events)) // events index of each publish
	var failed []int
	for i, ev := range events {
		if ev.UserID == uuid.Nil {
			continue
		}
		payload, err := encode(eventType, ev.Data, p.now())
		if err != nil {
			failed = append(failed, i)
			continue
		}
		users, payloads, pos = append(users, ev.UserID), append(payloads, payload), append(pos, i)
	}
	var local []int
	var err error
	if pb, ok := p.broker.(PipelineBroker); ok {
		local, err = pb.PublishPipelined(ctx, users, payloads)
	} else {
		for k := range users {
			if _, e := p.broker.Publish(ctx, users[k], payloads[k]); e != nil {
				local, err = append(local, k), e
			}
		}
	}
	for _, k := range local {
		failed = append(failed, pos[k])
	}
	return failed, err
}
