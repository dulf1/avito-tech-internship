package domain

import "context"

type Event struct {
	Type    string
	Payload map[string]any
}

type EventBus interface {
	Publish(ctx context.Context, e Event)
}
