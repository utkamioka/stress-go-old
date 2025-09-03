package cpu

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

// GenerateLoad は指定されたCPUコア数で負荷を生成します。
//
// 引数:
//
//	ctx       - 負荷生成の制御に使用するコンテキスト
//	coreCount - 使用するCPUコア数。0の場合は全CPUコアを使用
func GenerateLoad(ctx context.Context, coreCount int) {
	// If coreCount is 0, use all available CPU cores
	if coreCount == 0 {
		coreCount = runtime.NumCPU()
	}
	
	fmt.Printf("[CPU] Starting load generation on %d cores\n", coreCount)
	
	// Set GOMAXPROCS to limit OS thread count
	oldMaxProcs := runtime.GOMAXPROCS(coreCount)
	defer runtime.GOMAXPROCS(oldMaxProcs)

	var wg sync.WaitGroup
	
	// Start goroutine for each CPU core
	for i := 0; i < coreCount; i++ {
		wg.Add(1)
		go func(coreID int) {
			defer wg.Done()
			generateCoreLoad(ctx, coreID)
		}(i)
	}
	
	wg.Wait()
	fmt.Printf("[CPU] Load generation completed\n")
}

// generateCoreLoad generates load on a single CPU core.
func generateCoreLoad(ctx context.Context, coreID int) {
	fmt.Printf("[CPU] Starting load generation on core %d\n", coreID)
	
	// Execute maximum CPU-intensive calculations
	var result uint64
	checkInterval := uint64(50000000) // Check context every 50M iterations
	
	for {
		// Pure integer operations for maximum CPU utilization
		for i := uint64(0); i < checkInterval; i++ {
			// Mix of operations to maximize CPU usage
			result = result*1103515245 + 12345 // Linear congruential generator
			result ^= result >> 21
			result ^= result << 35
			result ^= result >> 4
			result += i * 31
		}
		
		// Check context only after many iterations
		select {
		case <-ctx.Done():
			fmt.Printf("[CPU] Stopping load generation on core %d\n", coreID)
			// Use result to prevent optimization
			if result == 0 {
				fmt.Printf("[CPU] Final result: %d\n", result)
			}
			return
		default:
			// Continue without any pause
		}
	}
}