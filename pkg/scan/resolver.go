package scan

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// ConcurrentDNSResolver represents DNS resolver
type ConcurrentDNSResolver struct {
	sem *semaphore.Weighted
}

// NewResolver takes the number of concurrent lookups and intantiates weighted semaphores
func NewResolver(maxConcurrent int64) *ConcurrentDNSResolver {
	return &ConcurrentDNSResolver{
		sem: semaphore.NewWeighted(maxConcurrent),
	}
}

// ResolveConcurrently will resolve IP to Names concurrently
func (r *ConcurrentDNSResolver) ResolveConcurrently(events []*NetworkEvent) {
	var wg sync.WaitGroup

	for _, event := range events {
		wg.Add(1)

		go func(e *NetworkEvent) {
			defer wg.Done()
			_ = r.sem.Acquire(context.Background(), 1)
			defer r.sem.Release(1)

			if e.RemoteIP != "" && isValidIPv4(e.RemoteIP) {
				e.RemoteDomain = performDNSLookup(e.RemoteIP)
			}
		}(event)
	}

	wg.Wait()
}
