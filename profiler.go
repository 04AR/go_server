package main

import (
	"fmt"
	"log"
	"runtime"
    "time"
	"net/http"

	_ "net/http/pprof" // For profiling
)

// ---- Memory Profiler Helper ----
func printMem() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[MEM] Alloc = %v KiB", m.Alloc/1024)
	fmt.Printf("\tTotalAlloc = %v KiB", m.TotalAlloc/1024)
	fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
	fmt.Printf("\tGoroutines = %v\n", runtime.NumGoroutine())
}

func InitProfiler(port string) {

	go func() {
		for {
			time.Sleep(5 * time.Second)
			printMem()
			
		}
	}()
	go func() {
		if port == "" {
			port = ":6060" // default value
		}	
		log.Printf("pprof listening on %s", port)
		log.Println(http.ListenAndServe(port, nil))
	}()
}