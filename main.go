package main

import (
	"fmt"
	"time"
)

func main() {
	//server := &Server{listenerAddress: ":8090"}
	//server.Start()

	cache := NewCache()
	defer cache.Stop()

	cache.Set("5", "dog", 10)
	cache.Set("1", "cat", 1)

	val1, _ := cache.Get("5")
	fmt.Printf("get 5, expected: dog, actual: %v\n", val1)

	time.Sleep(2 * time.Second)
	currentTtl1 := cache.Ttl("5")
	fmt.Printf("ttl 5, old=%v, new=%v\n", 10, currentTtl1)

	val2, _ := cache.Get("1")
	fmt.Printf("get 1, expected: cat, actual: %v\n", val2)

	time.Sleep(2 * time.Second)
	val2, _ = cache.Get("1")
	fmt.Printf("get 1, expected: empty string, actual: %v\n", val2)


	fmt.Print(cache.Stats())
}
