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
	done            atomic.Bool
	ctx             context.Context
	cancel          context.CancelFunc
	refreshInterval time.Duration

	timeBasedStatefulService *timeBasedStatefulService[P, V]
}

func (r *refreshExpiryTimeGoroutine[P, V]) Start() {
	if r.done.Load() {
		return
	}

	go func() {
		timer := time.NewTicker(r.refreshInterval)
		defer timer.Stop()
	loop:
		for !r.done.Load() {
			select {
			case <-r.ctx.Done():
				break loop
			case <-timer.C:
				now := time.Now()
				if now.After(r.timeBasedStatefulService.expire) && atomic.LoadInt32(&r.timeBasedStatefulService.count) == 0 {
					break loop
				}

				// 续期
				if now.Add(r.refreshInterval).After(r.timeBasedStatefulService.expire) {
					r.timeBasedStatefulService.expire.Add(r.timeBasedStatefulService.config.interval)
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
	if r.done.Swap(true) {
		return
	}
	r.cancel()
}

func newRefreshExpiryTimeGoroutine[P StatefulServiceParam, V StatefulService](ctx context.Context, timeBasedStatefulService *timeBasedStatefulService[P, V], refreshInterval time.Duration) *refreshExpiryTimeGoroutine[P, V] {
	ctx, cancel := context.WithCancel(ctx)
	return &refreshExpiryTimeGoroutine[P, V]{
		ctx:                      ctx,
		cancel:                   cancel,
		refreshInterval:          refreshInterval,
		timeBasedStatefulService: timeBasedStatefulService,
	}
}

type timeBasedStatefulService[P StatefulServiceParam, V StatefulService] struct {
	config *timeBasedStatefulServiceConfig

	count int32

	expire time.Time // expire time

	p P

	v V

	pool *TimeBasedStatefulServicePool[P, V]

	ctx context.Context
}

func (t *timeBasedStatefulService[P, V]) add() {
	atomic.AddInt32(&t.count, 1)
}

func (t *timeBasedStatefulService[P, V]) sub() {
	atomic.AddInt32(&t.count, -1)
}

func newTimeBasedStatefulService[P StatefulServiceParam, V StatefulService](ctx context.Context, p P, v V, pool *TimeBasedStatefulServicePool[P, V], opts ...options.Option[*timeBasedStatefulServiceConfig]) *timeBasedStatefulService[P, V] {
	timeBasedStatefulService := &timeBasedStatefulService[P, V]{
		config: &timeBasedStatefulServiceConfig{
			interval: 10 * time.Minute,
		},
		count: 1,
		p:     p,
		v:     v,
		pool:  pool,
		ctx:   ctx,
	}
	for _, opt := range opts {
		opt(timeBasedStatefulService.config)
	}

	timeBasedStatefulService.expire = time.Now().Add(timeBasedStatefulService.config.interval)

	newRefreshExpiryTimeGoroutine(ctx, timeBasedStatefulService, timeBasedStatefulService.config.interval/2).Start()

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
