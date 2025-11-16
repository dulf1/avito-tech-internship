package async

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Task func(ctx context.Context)

type WorkerPool struct {
	tasks  chan Task
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	log    *zap.Logger
}

func NewWorkerPool(parent context.Context, size int, log *zap.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(parent)
	p := &WorkerPool{
		tasks:  make(chan Task),
		ctx:    ctx,
		cancel: cancel,
		log:    log,
	}

	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return p
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}

			safeCtx, cancel := context.WithTimeout(p.ctx, 2*time.Second)
			func() {
				defer func() {
					if r := recover(); r != nil {
						p.log.Error("task panicked", zap.Any("panic", r))
					}
				}()
				task(safeCtx)
			}()
			cancel()
		}
	}
}

func (p *WorkerPool) Submit(task Task) {
	select {
	case <-p.ctx.Done():
		return
	case p.tasks <- task:
	}
}

func (p *WorkerPool) Shutdown() {
	p.cancel()
	close(p.tasks)
	p.wg.Wait()
}
