package refinery

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/rig"
)

func TestAcquireMainPushSlot_ConcurrentSingleWriter(t *testing.T) {
	var mu sync.Mutex
	currentHolder := ""

	e := &Engineer{
		rig: &rig.Rig{Name: "testrig"},
		mergeSlotEnsureExists: func() (string, error) {
			return "merge-slot", nil
		},
		mergeSlotAcquire: func(holder string, addWaiter bool) (*beads.MergeSlotStatus, error) {
			mu.Lock()
			defer mu.Unlock()
			if currentHolder == "" {
				currentHolder = holder
				return &beads.MergeSlotStatus{ID: "merge-slot", Available: true, Holder: holder}, nil
			}
			return &beads.MergeSlotStatus{ID: "merge-slot", Available: false, Holder: currentHolder}, nil
		},
		mergeSlotRelease: func(holder string) error {
			mu.Lock()
			defer mu.Unlock()
			if currentHolder != holder {
				return fmt.Errorf("holder mismatch: %s != %s", currentHolder, holder)
			}
			currentHolder = ""
			return nil
		},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var successCount int
	var failCount int
	var countMu sync.Mutex

	tryAcquire := func() {
		defer wg.Done()
		<-start
		holder, err := e.acquireMainPushSlot()
		if err != nil {
			countMu.Lock()
			failCount++
			countMu.Unlock()
			return
		}
		countMu.Lock()
		successCount++
		countMu.Unlock()
		time.Sleep(25 * time.Millisecond)
		if releaseErr := e.mergeSlotRelease(holder); releaseErr != nil {
			t.Errorf("release failed: %v", releaseErr)
		}
	}

	wg.Add(2)
	go tryAcquire()
	go tryAcquire()
	close(start)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly one successful slot acquisition, got %d", successCount)
	}
	if failCount != 1 {
		t.Fatalf("expected exactly one failed slot acquisition, got %d", failCount)
	}
}
