// Command hog maps memorytest.Ballast bytes, holds itself to the memory
// of the lys-docthumb request on its command line past them as
// lys-docthumb does, and allocates the bytes memorytest.Allocate names
// in its environment, or twice the request's memory, memorytest.Step at
// a time, keeping each. It exits 0 having allocated them all.
package main

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/dochelper/memorytest"
)

func main() {
	held, err := hog(os.Args[1:], os.Getenv(memorytest.Allocate))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hog:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "hog: allocated", held, "bytes")
}

// hog allocates allocate bytes, or twice the memory of the request
// args for "", past the ballast within that memory, and is how many it
// holds.
func hog(args []string, allocate string) (int64, error) {
	req, err := dochelper.ParseRequest(args)
	if err != nil {
		return 0, err
	}
	n := 2 * req.Memory
	if allocate != "" {
		if n, err = strconv.ParseInt(allocate, 10, 64); err != nil {
			return 0, fmt.Errorf("%s=%q: %w", memorytest.Allocate, allocate, err)
		}
	}
	if _, err := unix.Mmap(-1, 0, memorytest.Ballast, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANONYMOUS|unix.MAP_NORESERVE); err != nil {
		return 0, fmt.Errorf("mapping the ballast: %w", err)
	}
	if err := dochelper.LimitMemory(req.Memory); err != nil {
		return 0, err
	}
	held := make([][]byte, 0, n/memorytest.Step)
	for range cap(held) {
		held = append(held, make([]byte, memorytest.Step))
	}
	return int64(len(held)) * memorytest.Step, nil
}
