//+build !go1.14

package main

import (
	"hash/fnv"
	"sync"
)

func newHasher() func(string) uint64 {
	var mu sync.Mutex
	hasher := fnv.New64a()
	return func(s string) uint64 {
		mu.Lock()
		defer mu.Unlock()
		hasher.Reset()
		hasher.Write([]byte(s))
		return hasher.Sum64()
	}
}
