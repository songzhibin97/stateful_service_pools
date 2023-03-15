package stateful_service_pools

// StatefulService is the interface that wraps the basic methods of a stateful service.
type StatefulService interface {
	Close()
}

type StatefulServiceParam interface {
	String() string
}

type NewStatefulService[P any, V StatefulService] interface {
	NewStatefulService(P) V
}

// StatefulServicePool is the interface that wraps the basic methods of a stateful service pool.
type StatefulServicePool[P any, V any] interface {
	Get(P) V
	Put(P, V)
	Del(P)
}
