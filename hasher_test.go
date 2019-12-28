//+build go1.14

package main

import (
	"hash/fnv"
	"hash/maphash"
	"testing"
)

var sink uint64

const testKey = "hello, world"

func BenchmarkMaphash(b *testing.B) {
	var h maphash.Hash
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.WriteString(testKey)
		sink = h.Sum64()
	}
}

func BenchmarkFnv(b *testing.B) {
	h := fnv.New64a()
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.Write([]byte(testKey))
		sink = h.Sum64()
	}
}
