package stateful_service_pools

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/go-baseutils/base/bmap"

	"github.com/songzhibin97/go-baseutils/base/options"
)

type timeBasedStatefulServiceConfig struct {
	interval time.Duration
}

func SetTimeBasedStatefulServiceInterval(interval time.Duration) options.Option[*timeBasedStatefulServiceConfig] {
	return func(o *timeBasedStatefulServiceConfig) {
		if interval <= 0 {
			return
		}
		o.interval = interval
	}
}

type refreshExpiryTimeGoroutine[P StatefulServiceParam, V StatefulService] struct {
	ctx context.Context

	timeBasedStatefulService *timeBasedStatefulService[P, V]
}

func (r *refreshExpiryTimeGoroutine[P, V]) Start() {
	if r.timeBasedStatefulService.done.Load() {
		return
	}

	go func() {
		ticker := time.NewTicker(r.timeBasedStatefulService.config.interval)
		defer ticker.Stop()
		pre := time.Now()
	loop:
		for !r.timeBasedStatefulService.done.Load() {
			select {
			case <-r.ctx.Done():
				break loop
			case timer := <-ticker.C:
				if pre.After(timer) {
					continue
				}
				break loop
			case number := <-r.timeBasedStatefulService.event:
				val := atomic.AddInt32(&r.timeBasedStatefulService.count, number)
				if val <= 0 {
					pre = time.Now()
					ticker.Reset(r.timeBasedStatefulService.config.interval)
				} else {
					ticker.Stop()
				}
			}
		}

		// 已经过期了并且没有引用了
		// 从维护的节点摘除
		r.timeBasedStatefulService.pool.Del(r.timeBasedStatefulService.p)

		// 关闭监视
		r.Stop()
	}()
}

func (r *refreshExpiryTimeGoroutine[P, V]) Stop() {
	if r.timeBasedStatefulService.done.Swap(true) {
		return
	}
	r.timeBasedStatefulService.cancel()
	close(r.timeBasedStatefulService.event)
}

func newRefreshExpiryTimeGoroutine[P StatefulServiceParam, V StatefulService](ctx context.Context, timeBasedStatefulService *timeBasedStatefulService[P, V]) *refreshExpiryTimeGoroutine[P, V] {
	return &refreshExpiryTimeGoroutine[P, V]{
		ctx:                      ctx,
		timeBasedStatefulService: timeBasedStatefulService,
	}
}

type timeBasedStatefulService[P StatefulServiceParam, V StatefulService] struct {
	done atomic.Bool

	config *timeBasedStatefulServiceConfig

	count int32

	p P

	v V

	pool *TimeBasedStatefulServicePool[P, V]

	ctx context.Context

	cancel context.CancelFunc

	event chan int32
}

func (t *timeBasedStatefulService[P, V]) add() {
	if t.done.Load() {
		return
	}
	t.event <- 1
}

func (t *timeBasedStatefulService[P, V]) sub() {
	if t.done.Load() {
		return
	}
	t.event <- -1
}

func newTimeBasedStatefulService[P StatefulServiceParam, V StatefulService](ctx context.Context, p P, v V, pool *TimeBasedStatefulServicePool[P, V], opts ...options.Option[*timeBasedStatefulServiceConfig]) *timeBasedStatefulService[P, V] {
	ctx, cancel := context.WithCancel(ctx)

	timeBasedStatefulService := &timeBasedStatefulService[P, V]{
		config: &timeBasedStatefulServiceConfig{
			interval: 10 * time.Minute,
		},
		count:  0,
		p:      p,
		v:      v,
		pool:   pool,
		ctx:    ctx,
		cancel: cancel,
		event:  make(chan int32, 10),
	}
	for _, opt := range opts {
		opt(timeBasedStatefulService.config)
	}

	timeBasedStatefulService.add()

	newRefreshExpiryTimeGoroutine(ctx, timeBasedStatefulService).Start()

	return timeBasedStatefulService
}

type TimeBasedStatefulServicePool[P StatefulServiceParam, V StatefulService] struct {
	ctx context.Context

	safeMap bmap.AnyBMap[string, *timeBasedStatefulService[P, V]]

	service NewStatefulService[P, V]

	opts []options.Option[*timeBasedStatefulServiceConfig]
}

func (t *TimeBasedStatefulServicePool[P, V]) Get(p P) V {
	v, ok := t.safeMap.Get(p.String())
	if ok {
		v.add()
		return v.v
	}
	nv := t.service.NewStatefulService(p)
	t.safeMap.Put(p.String(), newTimeBasedStatefulService[P, V](t.ctx, p, nv, t, t.opts...))
	return nv
}

func (t *TimeBasedStatefulServicePool[P, V]) Put(p P, v V) {
	ov, ok := t.safeMap.Get(p.String())
	if !ok {
		t.safeMap.PuTIfAbsent(p.String(), newTimeBasedStatefulService[P, V](t.ctx, p, v, t, t.opts...))
		return
	}
	ov.sub()
}

func (t *TimeBasedStatefulServicePool[P, V]) Del(p P) {
	ov, ok := t.safeMap.Get(p.String())
	if !ok {
		return
	}
	ov.v.Close()
	t.safeMap.Delete(p.String())
}

func NewTimeBasedStatefulServicePool[P StatefulServiceParam, V StatefulService](ctx context.Context, service NewStatefulService[P, V], opts ...options.Option[*timeBasedStatefulServiceConfig]) *TimeBasedStatefulServicePool[P, V] {
	return &TimeBasedStatefulServicePool[P, V]{
		safeMap: bmap.NewSafeAnyBMap[string, *timeBasedStatefulService[P, V]](),
		service: service,
		opts:    opts,
		ctx:     ctx,
	}
}
