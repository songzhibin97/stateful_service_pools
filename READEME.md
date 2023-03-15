# stateful_service_pools
有状态服务池

## quick start
```shell
go get github.com/songzhibin97/stateful_service_pools
```

The StatefulService、StatefulServiceParam、NewStatefulService interface needs to be implemented beforehand.

Simple test!

```go
package main

import (
	"context"
	"github.com/songzhibin97/stateful_service_pools"
	"time"
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

func main() {
	service := &mockNewStatefulService{}
	pool := stateful_service_pools.NewTimeBasedStatefulServicePool[mockStatefulServiceParam, *mockStatefulService](context.Background(), service, stateful_service_pools.SetTimeBasedStatefulServiceInterval(time.Second*10))
	test1 := pool.Get("test1")

	pool.Put("test1", test1)
}

```