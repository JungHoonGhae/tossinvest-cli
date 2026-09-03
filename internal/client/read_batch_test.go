package client

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunReadBatchRunsConcurrentlyAndReportsInDeclarationOrder(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runReadBatch(
			readTask{label: "first", run: func() error {
				started <- "first"
				<-release
				return errors.New("first failure")
			}},
			readTask{label: "second", run: func() error {
				started <- "second"
				<-release
				return errors.New("second failure")
			}},
		)
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("read tasks did not start concurrently")
		}
	}
	close(release)
	if err := <-done; err == nil || !strings.Contains(err.Error(), "first: first failure") {
		t.Fatalf("error = %v, want first declared failure", err)
	}
}
