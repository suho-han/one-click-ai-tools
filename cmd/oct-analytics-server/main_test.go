package main

import "testing"

func TestLoopbackAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8765", "[::1]:8766"} {
		if err := loopbackAddress(addr); err != nil {
			t.Fatalf("valid %s: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:8765", "[::]:8766", "localhost:8765", "192.0.2.8:8765", "127.0.0.1:0", "127.0.0.1:abc", "127.0.0.1:65536", ":8765"} {
		if err := loopbackAddress(addr); err == nil {
			t.Fatalf("unsafe bind accepted: %s", addr)
		}
	}
}
