package core

import (
	"sort"
	"strconv"
)

// Value is a configuration value as JavaScript holds one read from YAML or JSON: nil, bool,
// float64, string, []Value or *Object.
type Value = any

// Object is a plain JavaScript object: string keys, enumerated in JavaScript's order, which puts
// array-index keys first in numeric order and the rest in the order they were added.
type Object struct {
	keys []string
	vals map[string]Value
}

func NewObject() *Object { return &Object{vals: map[string]Value{}} }

// Get returns the value at key and whether the key is present.
func (o *Object) Get(key string) (Value, bool) {
	if o == nil {
		return nil, false
	}
	v, ok := o.vals[key]
	return v, ok
}

// Value returns the value at key, or nil.
func (o *Object) Value(key string) Value {
	v, _ := o.Get(key)
	return v
}

func (o *Object) Has(key string) bool {
	_, ok := o.Get(key)
	return ok
}

func (o *Object) Set(key string, v Value) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

func (o *Object) Delete(key string) {
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

func arrayIndex(key string) (uint64, bool) {
	if key == "" || len(key) > 10 || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(key, 10, 64)
	if err != nil || n >= 4294967295 {
		return 0, false
	}
	return n, true
}

// Keys returns the keys in the order Object.keys gives them.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	var index, rest []string
	for _, k := range o.keys {
		if _, ok := arrayIndex(k); ok {
			index = append(index, k)
		} else {
			rest = append(rest, k)
		}
	}
	if len(index) == 0 {
		return rest
	}
	sort.Slice(index, func(a, b int) bool {
		x, _ := arrayIndex(index[a])
		y, _ := arrayIndex(index[b])
		return x < y
	})
	return append(index, rest...)
}

// Clone copies the object one level deep, as `{ ...o }` does.
func (o *Object) Clone() *Object {
	c := NewObject()
	for _, k := range o.Keys() {
		c.Set(k, o.vals[k])
	}
	return c
}

// Assign copies the keys of src into o, as Object.assign or a spread does.
func (o *Object) Assign(src *Object) *Object {
	for _, k := range src.Keys() {
		o.Set(k, src.vals[k])
	}
	return o
}

// Obj returns v as an object, or nil when it is not one.
func Obj(v Value) *Object {
	o, _ := v.(*Object)
	return o
}

// Section returns config[key] when it is an object, or nil.
func (o *Object) Section(key string) *Object { return Obj(o.Value(key)) }

// Truthy is JavaScript's ToBoolean.
func Truthy(v Value) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x == x && x != 0
	case string:
		return x != ""
	}
	return true
}
