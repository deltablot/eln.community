package app

import (
	"fmt"
	"testing"
	"time"
)

func TestInMemoryCache_SetAndGet(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	// Test basic set and get
	cache.Set("key1", "value1")

	value, found := cache.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %s", value)
	}

	// Test non-existent key
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("Expected not to find nonexistent key")
	}
}

func TestInMemoryCache_Expiration(t *testing.T) {
	cache := NewInMemoryCache[string](100 * time.Millisecond)
	defer cache.Stop()

	cache.Set("key1", "value1")

	// Should be found immediately
	_, found := cache.Get("key1")
	if !found {
		t.Error("Expected to find key1 immediately")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not be found after expiration
	_, found = cache.Get("key1")
	if found {
		t.Error("Expected key1 to be expired")
	}
}

func TestInMemoryCache_Clear(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
}

func TestInMemoryCache_Size(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	if cache.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", cache.Size())
	}

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}
}

func TestInMemoryCache_Keys(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	keys := cache.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all keys are present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	if !keyMap["key1"] || !keyMap["key2"] || !keyMap["key3"] {
		t.Error("Not all expected keys found")
	}
}

func TestInMemoryCache_GetMultiple(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	found, missing := cache.GetMultiple([]string{"key1", "key2", "key3"})

	if len(found) != 2 {
		t.Errorf("Expected 2 found items, got %d", len(found))
	}
	if len(missing) != 1 {
		t.Errorf("Expected 1 missing item, got %d", len(missing))
	}

	if found["key1"] != "value1" {
		t.Errorf("Expected value1, got %s", found["key1"])
	}
	if found["key2"] != "value2" {
		t.Errorf("Expected value2, got %s", found["key2"])
	}
	if missing[0] != "key3" {
		t.Errorf("Expected key3 to be missing, got %s", missing[0])
	}
}

func TestInMemoryCache_SetMultiple(t *testing.T) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	items := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	cache.SetMultiple(items)

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	for key, expectedValue := range items {
		value, found := cache.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s", expectedValue, value)
		}
	}
}

func TestInMemoryCache_Has(t *testing.T) {
	cache := NewInMemoryCache[string](100 * time.Millisecond)
	defer cache.Stop()

	cache.Set("key1", "value1")

	if !cache.Has("key1") {
		t.Error("Expected Has to return true for key1")
	}

	if cache.Has("nonexistent") {
		t.Error("Expected Has to return false for nonexistent key")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	if cache.Has("key1") {
		t.Error("Expected Has to return false for expired key")
	}
}

func TestInMemoryCache_Cleanup(t *testing.T) {
	// Create cache with short cleanup interval
	cache := NewInMemoryCacheWithCleanup[string](100*time.Millisecond, 200*time.Millisecond)
	defer cache.Stop()

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	// Wait for expiration and cleanup
	time.Sleep(400 * time.Millisecond)

	// After cleanup, expired items should be removed
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", cache.Size())
	}
}

func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewInMemoryCache[int](1 * time.Hour)
	defer cache.Stop()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(val int) {
			cache.Set("key", val)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have a value (last write wins)
	_, found := cache.Get("key")
	if !found {
		t.Error("Expected to find key after concurrent writes")
	}

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			cache.Get("key")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestInMemoryCache_GenericTypes(t *testing.T) {
	// Test with int
	intCache := NewInMemoryCache[int](1 * time.Hour)
	defer intCache.Stop()

	intCache.Set("num", 42)
	value, found := intCache.Get("num")
	if !found || value != 42 {
		t.Error("Failed to store/retrieve int")
	}

	// Test with struct
	type Person struct {
		Name string
		Age  int
	}

	personCache := NewInMemoryCache[Person](1 * time.Hour)
	defer personCache.Stop()

	person := Person{Name: "Alice", Age: 30}
	personCache.Set("person1", person)

	retrieved, found := personCache.Get("person1")
	if !found || retrieved.Name != "Alice" || retrieved.Age != 30 {
		t.Error("Failed to store/retrieve struct")
	}
}

func BenchmarkInMemoryCache_Set(b *testing.B) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", "value")
	}
}

func BenchmarkInMemoryCache_Get(b *testing.B) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	cache.Set("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

func BenchmarkInMemoryCache_GetMultiple(b *testing.B) {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	for i := 0; i < 100; i++ {
		cache.Set("key", "value")
	}

	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		keys[i] = "key"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetMultiple(keys)
	}
}

// Example: Basic string cache
func ExampleInMemoryCache_basic() {
	cache := NewInMemoryCache[string](1 * time.Hour)
	defer cache.Stop()

	cache.Set("greeting", "Hello, World!")

	if value, found := cache.Get("greeting"); found {
		fmt.Println(value)
	}
	// Output: Hello, World!
}

// Example: Struct cache
func ExampleInMemoryCache_struct() {
	type User struct {
		ID   int
		Name string
	}

	cache := NewInMemoryCache[User](1 * time.Hour)
	defer cache.Stop()

	user := User{ID: 1, Name: "Alice"}
	cache.Set("user:1", user)

	if cached, found := cache.Get("user:1"); found {
		fmt.Printf("%s (ID: %d)\n", cached.Name, cached.ID)
	}
	// Output: Alice (ID: 1)
}

// Example: Batch operations
func ExampleInMemoryCache_batch() {
	cache := NewInMemoryCache[int](1 * time.Hour)
	defer cache.Stop()

	// Set multiple values
	items := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	cache.SetMultiple(items)

	// Get multiple values
	found, missing := cache.GetMultiple([]string{"one", "two", "four"})

	fmt.Printf("Found: %d items\n", len(found))
	fmt.Printf("Missing: %v\n", missing)
	// Output:
	// Found: 2 items
	// Missing: [four]
}

// Example: Pointer types
func ExampleInMemoryCache_pointers() {
	type Config struct {
		Setting string
		Value   int
	}

	cache := NewInMemoryCache[*Config](1 * time.Hour)
	defer cache.Stop()

	config := &Config{Setting: "max_connections", Value: 100}
	cache.Set("config", config)

	if cached, found := cache.Get("config"); found {
		fmt.Printf("%s: %d\n", cached.Setting, cached.Value)
	}
	// Output: max_connections: 100
}

// Example: Map values
func ExampleInMemoryCache_maps() {
	cache := NewInMemoryCache[map[string]string](1 * time.Hour)
	defer cache.Stop()

	metadata := map[string]string{
		"author":  "Alice",
		"version": "1.0",
	}
	cache.Set("metadata", metadata)

	if cached, found := cache.Get("metadata"); found {
		fmt.Printf("Author: %s, Version: %s\n", cached["author"], cached["version"])
	}
	// Output: Author: Alice, Version: 1.0
}

// Example: Slice values
func ExampleInMemoryCache_slices() {
	cache := NewInMemoryCache[[]string](1 * time.Hour)
	defer cache.Stop()

	tags := []string{"golang", "cache", "performance"}
	cache.Set("tags", tags)

	if cached, found := cache.Get("tags"); found {
		fmt.Printf("Tags: %v\n", cached)
	}
}
