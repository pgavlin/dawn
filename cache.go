package dawn

import (
	"errors"
	"sync"

	"github.com/pgavlin/starlark-go/starlark"
)

// A cache provides a simple, concurrency-safe string -> value map for use by dawn programs.
type cache struct {
	m       sync.RWMutex
	entries map[string]starlark.Value

	onceM *starlark.Builtin
}

var builtin_cache = starlark.NewBuiltin("Cache", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	c := &cache{entries: map[string]starlark.Value{}}
	c.onceM = c.newOnce()
	return c, nil
})

func (c *cache) get(key string) (starlark.Value, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	v, ok := c.entries[key]
	return v, ok
}

// starlark
//
//	def once(key, callable):
//	    """
//	    once calls the given callable if and only if key is not present in the cache.
//
//	    The result is stored in the cache under the given key.
//
//	    Returns the result of the call or the cached value.
//	    """
//
//starlark:builtin
func (c *cache) once(thread *starlark.Thread, _ *starlark.Builtin, key string, function starlark.Callable) (starlark.Value, error) {
	if v, ok := c.get(key); ok {
		return v, nil
	}

	c.m.Lock()
	defer c.m.Unlock()

	if v, ok := c.entries[key]; ok {
		return v, nil
	}

	v, err := starlark.Call(thread, function, nil, nil)
	if err != nil {
		return nil, err
	}
	c.entries[key] = v

	return v, nil
}

func (c *cache) String() string        { return "cache" }
func (c *cache) Type() string          { return "cache" }
func (c *cache) Freeze()               {} // logically immutable
func (c *cache) Truth() starlark.Bool  { return starlark.True }
func (c *cache) Hash() (uint32, error) { return 0, errors.New("cache is not hashable") }

func (c *cache) Attr(name string) (starlark.Value, error) {
	if name != "once" {
		return nil, nil
	}
	return c.onceM, nil
}

func (c *cache) AttrNames() []string {
	return []string{"once"}
}
