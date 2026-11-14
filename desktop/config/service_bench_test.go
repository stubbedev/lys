package config

import (
	"testing"
)

var benchLocations = [...]string{"top", "bottom"}

func benchService(b *testing.B) *Service {
	b.Helper()
	svc := Load(b.TempDir(), DiscardDiagnostics)
	b.Cleanup(svc.Close)
	return svc
}

func BenchmarkSetByPath(b *testing.B) {
	svc := benchService(b)
	b.ReportAllocs()
	for i := range b.N {
		if err := svc.SetByPath("bar.location", benchLocations[i%2]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetByPathAfterAWrite(b *testing.B) {
	svc := benchService(b)
	b.ReportAllocs()
	for i := range b.N {
		b.StopTimer()
		if err := svc.SetByPath("bar.location", benchLocations[i%2]); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := svc.GetByPath("bar.location"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSave(b *testing.B) {
	svc := benchService(b)
	if err := svc.SetByPath("bar.location", "bottom"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for range b.N {
		if err := svc.Save(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSettingsWrite(b *testing.B) {
	svc := benchService(b)
	b.ReportAllocs()
	for i := range b.N {
		if err := svc.SetByPath("bar.location", benchLocations[i%2]); err != nil {
			b.Fatal(err)
		}
		if err := svc.Save(); err != nil {
			b.Fatal(err)
		}
		if _, err := svc.GetByPath("bar.location"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResetByPath(b *testing.B) {
	svc := benchService(b)
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		if err := svc.SetByPath("bar.location", "bottom"); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := svc.ResetByPath("bar.location"); err != nil {
			b.Fatal(err)
		}
	}
}
