package skylark

import (
	"fmt"
	"sync"
	"testing"
)

func TestSharedEngineSingleton(t *testing.T) {
	t.Cleanup(ResetSharedEnginesForTests)

	dir := t.TempDir()
	eng1, release1, err := AcquireEngine(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng2, release2, err := AcquireEngine(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if eng1 != eng2 {
		t.Fatal("expected same engine instance")
	}
	release1()
	release2()
}

func TestPreloadThenAcquire(t *testing.T) {
	t.Cleanup(ResetSharedEnginesForTests)

	dir := t.TempDir()
	if err := PreloadEngine(dir, nil); err != nil {
		t.Fatal(err)
	}
	eng, release, err := AcquireEngine(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if eng == nil {
		t.Fatal("expected engine")
	}
}

func TestConcurrentAcquire(t *testing.T) {
	t.Cleanup(ResetSharedEnginesForTests)

	dir := t.TempDir()
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eng, release, err := AcquireEngine(dir, nil)
			if err != nil {
				errs <- err
				return
			}
			defer release()
			if eng == nil {
				errs <- fmt.Errorf("nil engine")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
