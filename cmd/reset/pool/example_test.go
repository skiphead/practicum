// example_test.go
package pool

import (
	"fmt"
	"testing"
)

// generate:reset
type Buffer struct {
	data []byte
	len  int
}

func (b *Buffer) Reset() {
	b.data = b.data[:0]
	b.len = 0
}

func (b *Buffer) WriteString(s string) {
	b.data = append(b.data, s...)
	b.len += len(s)
}

func (b *Buffer) String() string {
	return string(b.data[:b.len])
}

func NewBuffer() *Buffer {
	return &Buffer{
		data: make([]byte, 0, 1024),
	}
}

func TestPoolBasicUsage(t *testing.T) {
	// Создаем пул с конструктором
	p := New(NewBuffer)

	// Получаем объект из пула
	buf := p.Get()
	buf.WriteString("Hello, World!")
	fmt.Println(buf.String()) // "Hello, World!"

	// Возвращаем объект в пул
	p.Put(buf)

	// Снова получаем объект (он будет сброшен)
	buf2 := p.Get()
	fmt.Println(buf2.String()) // ""
	buf2.WriteString("Reused!")
	fmt.Println(buf2.String()) // "Reused!"
}

type ComplexStruct struct {
	id      int
	name    string
	items   []int
	options map[string]any
}

func (cs *ComplexStruct) Reset() {
	cs.id = 0
	cs.name = ""
	cs.items = cs.items[:0]
	clear(cs.options)
}

func NewComplexStruct() *ComplexStruct {
	return &ComplexStruct{
		items:   make([]int, 0, 10),
		options: make(map[string]any),
	}
}

func TestPoolWithComplexStruct(t *testing.T) {
	// Пул работает с любой структурой, имеющей метод Reset()
	pool := New(NewComplexStruct)

	obj := pool.Get()
	obj.id = 1
	obj.name = "test"
	obj.items = append(obj.items, 1, 2, 3)
	obj.options["key"] = "value"

	pool.Put(obj)

	// Новый объект будет сброшен
	obj2 := pool.Get()
	if obj2.id != 0 || obj2.name != "" || len(obj2.items) != 0 || len(obj2.options) != 0 {
		t.Error("Object was not properly reset")
	}
}
