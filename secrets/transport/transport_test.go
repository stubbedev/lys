package transport

import (
	"bytes"
	"crypto/aes"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestAnAESSessionRoundTripsAndRefusesBadPadding(t *testing.T) {
	a, err := NewAES(bytes.Repeat([]byte{7}, PrimeLen))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"", "x", "exactly sixteen!", "a longer secret than one block"} {
		params, value, err := a.Encode([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		got, err := a.Decode(params, value)
		if err != nil || string(got) != secret {
			t.Fatalf("%q: %q %v", secret, got, err)
		}
	}
	if _, err := a.Decode(make([]byte, aes.BlockSize), make([]byte, aes.BlockSize)); err == nil {
		t.Fatal("a zero block decoded")
	}
	if _, err := (Plain{}).Decode([]byte{1}, []byte("x")); err == nil {
		t.Fatal("a plain secret with parameters decoded")
	}
}

func TestAnOversizedSecretIsRefusedWithoutAllocatingIt(t *testing.T) {
	a, err := NewAES(bytes.Repeat([]byte{7}, PrimeLen))
	if err != nil {
		t.Fatal(err)
	}
	oversized := make([]byte, maxCiphertextLen+aes.BlockSize)
	iv := make([]byte, aes.BlockSize)
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{"AES encode", func() error { _, _, err := a.Encode(oversized[:MaxSecretLen+1]); return err }, "plaintext is " + strconv.Itoa(MaxSecretLen+1)},
		{"AES decode", func() error { _, err := a.Decode(iv, oversized); return err }, "ciphertext is " + strconv.Itoa(len(oversized))},
		{"plain encode", func() error { _, _, err := Plain{}.Encode(oversized[:MaxSecretLen+1]); return err }, "plaintext is " + strconv.Itoa(MaxSecretLen+1)},
		{"plain decode", func() error { _, err := Plain{}.Decode(nil, oversized[:MaxSecretLen+1]); return err }, "value is " + strconv.Itoa(MaxSecretLen+1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := tc.call()
			runtime.ReadMemStats(&after)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one with %q", err, tc.want)
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= MaxSecretLen/2 {
				t.Errorf("refusing allocated %d bytes", allocated)
			}
		})
	}
}

func TestTheLongestSecretRoundTrips(t *testing.T) {
	a, err := NewAES(bytes.Repeat([]byte{7}, PrimeLen))
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, MaxSecretLen)
	secret[len(secret)-1] = 1
	for _, tr := range []Transport{a, Plain{}} {
		params, value, err := tr.Encode(secret)
		if err != nil {
			t.Fatalf("%T encode: %v", tr, err)
		}
		got, err := tr.Decode(params, value)
		if err != nil || !bytes.Equal(got, secret) {
			t.Fatalf("%T decode: %d bytes, %v", tr, len(got), err)
		}
	}
}

func FuzzAESDecode(f *testing.F) {
	a := AES{key: make([]byte, aes.BlockSize)}
	f.Add(make([]byte, 16), make([]byte, 32))
	f.Fuzz(func(t *testing.T, params, value []byte) {
		_, _ = a.Decode(params, value)
	})
}
