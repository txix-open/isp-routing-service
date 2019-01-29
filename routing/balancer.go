package routing

import "sync"

type RoundRobinBalancer struct {
	next     int
	addrList []string
	length   int
	mu       sync.Mutex
}

func (rrb *RoundRobinBalancer) Next() string {

	if rrb.length == 1 {
		return rrb.addrList[0]
	}

	rrb.mu.Lock()
	sc := rrb.addrList[rrb.next]
	rrb.next = (rrb.next + 1) % rrb.length
	rrb.mu.Unlock()
	return sc
}
