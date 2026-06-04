package cliutil

import (
	"sort"
	"sync"
	"testing"
	"time"
)

func TestAdaptiveLimiterWaitSerializesConcurrentReservations(t *testing.T) {
	limiter := NewAdaptiveLimiter(50)
	const workers = 4
	start := make(chan struct{})
	times := make([]time.Time, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			<-start
			limiter.Wait()
			times[index] = time.Now()
		}(i)
	}

	close(start)
	wg.Wait()
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	totalSpan := times[workers-1].Sub(times[0])
	if totalSpan < 45*time.Millisecond {
		t.Fatalf("concurrent waits were not serialized; total span = %s", totalSpan)
	}
}
