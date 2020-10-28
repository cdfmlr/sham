package sham

// Memory 是模拟的「内存」
// 这里认为「内存」就是一堆对象的集合（切片）
type Memory []Object

// Object 是「内存」中保存的「对象」，具体是啥都行。
type Object struct {
	Pid     string
	Content interface{}
}
