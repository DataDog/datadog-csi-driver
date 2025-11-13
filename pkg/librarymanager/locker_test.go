// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"sync"
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/stretchr/testify/require"
)

func TestLocker(t *testing.T) {
	l := librarymanager.NewLocker()
	goroutines := 20
	iterations := 100
	active := 0
	wg := sync.WaitGroup{}
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				l.Lock("same-key")
				active++
				require.Equal(t, 1, active, "there should only be one active go routine at any given point in time")
				active--
				l.Unlock("same-key")
			}
		}()
	}
	wg.Wait()
}
