package client

import (
	"fmt"
	"sync"
)

// readTask is an internal seam for strict aggregate reads: every task must
// succeed, but independent network requests should not add their latencies.
type readTask struct {
	label string
	run   func() error
}

// runReadBatch executes independent reads concurrently and reports failures in
// declaration order. That keeps observable errors deterministic even when the
// network completes requests in a different order.
func runReadBatch(tasks ...readTask) error {
	errs := make([]error, len(tasks))
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for i := range tasks {
		go func(i int) {
			defer wg.Done()
			errs[i] = tasks[i].run()
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return fmt.Errorf("%s: %w", tasks[i].label, err)
		}
	}
	return nil
}
