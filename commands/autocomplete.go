package commands

type AutocompleteCache[K comparable, T interface{}] struct {
	Name  string
	Cache map[K]T
}

// Get an item from the cache, returning an empty item of the cache type if not found
func (c *AutocompleteCache[K, T]) Get(key K) (item T, found bool) {
	if v, ok := c.Cache[key]; ok {
		return v, ok
	}
	var empty T
	return empty, false
}

// Set an item in the cache. Returns the cache for chaining
func (c *AutocompleteCache[K, T]) Set(key K, value T) *AutocompleteCache[K, T] {
	c.Cache[key] = value
	return c
}

// Clear the cache
func (c *AutocompleteCache[K, T]) Clear() {
	c.Cache = make(map[K]T)
}

// Delete an item from the cache
func (c *AutocompleteCache[K, T]) Delete(key K) {
	delete(c.Cache, key)
}

func NewAutocompleteCache[K comparable, T interface{}](name string) *AutocompleteCache[K, T] {
	return &AutocompleteCache[K, T]{
		Name:  name,
		Cache: make(map[K]T),
	}
}
