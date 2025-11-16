package async

import (
	"context"

	"go.uber.org/zap"

	"prservice/internal/domain"
)

type AsyncEventBus struct {
	pool *WorkerPool
	log  *zap.Logger
}

func NewAsyncEventBus(ctx context.Context, poolSize int, log *zap.Logger) *AsyncEventBus {
	return &AsyncEventBus{
		pool: NewWorkerPool(ctx, poolSize, log),
		log:  log,
	}
}

func (b *AsyncEventBus) Publish(ctx context.Context, e domain.Event) {
	b.pool.Submit(func(_ context.Context) {
		b.log.Info("domain_event",
			zap.String("type", e.Type),
			zap.Any("payload", e.Payload),
		)
	})
}

func (b *AsyncEventBus) Close() {
	b.pool.Shutdown()
}
