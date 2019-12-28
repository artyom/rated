//+build go1.14

package main

import (
	"hash/maphash"
	"sync"
)

func newHasher() func(string) uint64 {
	var mu sync.Mutex
	var hasher maphash.Hash
	return func(s string) uint64 {
		mu.Lock()
		defer mu.Unlock()
		hasher.Reset()
		hasher.WriteString(s)
		return hasher.Sum64()
	}
}
