package app

import (
	"errors"
	"fmt"
	"testing"
)

func TestAnAppThatEndsByClosingEndsWithoutAnError(t *testing.T) {
	if err := Ended(ErrClosed); err != nil {
		t.Errorf("a closed application ended with %v", err)
	}
	if err := Ended(fmt.Errorf("app: run: %w", ErrClosed)); err != nil {
		t.Errorf("a wrapped close ended with %v", err)
	}
	failed := errors.New("app: connect: no compositor")
	if err := Ended(failed); !errors.Is(err, failed) {
		t.Errorf("a failed application ended with %v, want %v", err, failed)
	}
}
