//go:build vm

package rfkill

import (
	"testing"
)

func TestVMAirplaneModeBlocksTheRadiosAndLetsThemGo(t *testing.T) {
	s := New(DevicePath)
	snap, err := s.Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Available() || snap.Airplane() {
		t.Fatalf("the VM's radios read available %v, airplane %v", snap.Available(), snap.Airplane())
	}
	if err := s.SetAirplane(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	if snap, err = s.Read(t.Context()); err != nil || !snap.Airplane() {
		t.Fatalf("airplane mode on reads %v, %v", snap.Airplane(), err)
	}
	if err := s.SetAirplane(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if snap, err = s.Read(t.Context()); err != nil || snap.Airplane() {
		t.Errorf("airplane mode off reads %v, %v", snap.Airplane(), err)
	}
}
