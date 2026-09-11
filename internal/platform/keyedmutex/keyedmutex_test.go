package keyedmutex

import (
	"sync"
	"testing"
	"time"
)

func TestMutex_SameKeySerializes(t *testing.T) {
	m := New()

	var (
		mu      sync.Mutex
		active  int
		maxSeen int
	)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			unlock := m.Lock("SBER")
			defer unlock()

			mu.Lock()
			active++
			if active > maxSeen {
				maxSeen = active
			}
			mu.Unlock()

			time.Sleep(time.Millisecond)

			mu.Lock()
			active--
			mu.Unlock()
		}()
	}
	wg.Wait()

	if maxSeen != 1 {
		t.Fatalf("max concurrent holders of the same key = %d, want 1", maxSeen)
	}
}

func TestMutex_DifferentKeysDoNotBlockEachOther(t *testing.T) {
	m := New()

	unlockA := m.Lock("SBER")
	defer unlockA()

	done := make(chan struct{})
	go func() {
		unlockB := m.Lock("GAZP")
		defer unlockB()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Lock on a different key blocked unexpectedly")
	}
}
