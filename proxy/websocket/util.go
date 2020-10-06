package websocket

import (
	"sync"

	"github.com/integration-system/isp-lib/v2/structure"
)

type RoundRobinAddrs struct {
	mu    *sync.Mutex
	addrs []structure.AddressConfiguration
	index int
}

func (u *RoundRobinAddrs) Get() structure.AddressConfiguration {
	u.mu.Lock()
	u.index = (u.index + 1) % len(u.addrs)
	addr := u.addrs[u.index]
	u.mu.Unlock()
	return addr
}

func NewRoundRobinAddrs(addrs []structure.AddressConfiguration) *RoundRobinAddrs {
	return &RoundRobinAddrs{
		addrs: addrs,
		index: -1,
		mu:    new(sync.Mutex),
	}
}
