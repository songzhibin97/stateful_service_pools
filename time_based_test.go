package stateful_service_pools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockStatefulService struct {
	name string
}

func (m mockStatefulService) Close() {

}

type mockStatefulServiceParam string

func (m mockStatefulServiceParam) String() string {
	return string(m)
}

type mockNewStatefulService struct {
}

func (m mockNewStatefulService) NewStatefulService(p mockStatefulServiceParam) *mockStatefulService {
	return &mockStatefulService{name: p.String()}
}

func TestTimeBasedStatefulServicePool(t *testing.T) {
	service := &mockNewStatefulService{}

	pool := NewTimeBasedStatefulServicePool[mockStatefulServiceParam, *mockStatefulService](context.Background(), service, SetTimeBasedStatefulServiceInterval(time.Second*10))

	_, ok := pool.safeMap.Get("test1")
	assert.Equal(t, false, ok)

	v := pool.Get("test1")
	time.Sleep(time.Second)

	assert.Equal(t, "test1", v.name)
	v2, ok := pool.safeMap.Get("test1")
	assert.Equal(t, true, ok)
	assert.Equal(t, int32(1), v2.count)

	v3 := pool.Get("test1")
	time.Sleep(time.Second)

	assert.Equal(t, v2.v, v3)

	v4, ok := pool.safeMap.Get("test1")
	assert.Equal(t, true, ok)
	assert.Equal(t, int32(2), v2.count)
	assert.Equal(t, v2.v, v4.v)

	pool.Put("test1", v)
	time.Sleep(time.Second)
	v5, ok := pool.safeMap.Get("test1")
	assert.Equal(t, true, ok)
	assert.Equal(t, int32(1), v5.count)

	pool.Put("test1", v)
	time.Sleep(time.Second)

	v6, ok := pool.safeMap.Get("test1")
	assert.Equal(t, true, ok)
	assert.Equal(t, int32(0), v6.count)

	time.Sleep(time.Second * 10)
	v7, ok := pool.safeMap.Get("test1")
	assert.Equal(t, false, ok)
	assert.Equal(t, true, v7 == nil)

	v8 := pool.Get("test2")

	time.Sleep(time.Second * 10)
	pool.Put("test2", v8)
	pool.Get("test2")
	time.Sleep(time.Second)

	v9, ok := pool.safeMap.Get("test2")
	assert.Equal(t, true, ok)
	assert.Equal(t, v9.v, v8)
}
