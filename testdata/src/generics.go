package testpkg

type Container[T any] struct {
	items []T
}

func (c *Container[T]) reset() {
	c.items = nil
}

func (c *Container[T]) Add(item T) {
	c.items = append(c.items, item)
}

func (c *Container[T]) Get(index int) T {
	return c.items[index]
}

func (c *Container[T]) Len() int {
	return len(c.items)
}
