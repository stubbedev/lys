package passwd

import "testing"

func UseSocket(t *testing.T, path string) {
	prev := socket
	socket = path
	t.Cleanup(func() { socket = prev })
}
