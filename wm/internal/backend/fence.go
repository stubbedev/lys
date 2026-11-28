package backend

import "golang.org/x/sys/unix"

type Fence int

const NoFence Fence = -1

func (f Fence) Valid() bool {
	return f >= 0
}

func (f Fence) Close() error {
	if !f.Valid() {
		return nil
	}
	return unix.Close(int(f))
}

func (f *Fence) Take() Fence {
	taken := *f
	*f = NoFence
	return taken
}

func (f *Fence) Drop() error {
	return f.Take().Close()
}
