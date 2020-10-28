package sham

import "sync"

// Device 是模拟的「IO设备」
// Inspired by *nix /dev
// 某一时刻只能有一个人在用，所以有锁
// 这个模拟里用 channel Input、Output 进行 IO
type Device struct {
	Id string
	sync.Mutex

	Input  chan Object
	Output chan Object
}

// read_only := make (<-chan int)
// write_only := make (chan<- int)
