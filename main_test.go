package main

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := newLimiter(10, 2, time.Second)
	results := make([]bool, 3)
	for i := 0; i < len(results); i++ {
		results[i] = l.allow("foo")
	}
	want := []bool{true, true, false}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Fatalf("got:\n%v\nwant:\n%v", results, want)
		}
	}
}

func TestBucket(t *testing.T) {
	var b bucket
	results := make([]bool, 3)
	for i := 0; i < len(results); i++ {
		results[i] = b.allow(time.Now(), 2, float64(time.Second))
	}
	want := []bool{true, true, false}
	for i := 0; i < len(results); i++ {
		if results[i] != want[i] {
			t.Logf("bucket: %+v", b)
			t.Fatalf("got:\n%v\nwant:\n%v", results, want)
		}
	}
}
