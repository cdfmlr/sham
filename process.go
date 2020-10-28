package sham

import (
	"context"
	"fmt"
)

type Runnable func(contextual *Contextual)

// Thread 线程：是一个可以在 CPU 里跑的东西。
type Thread struct {
	// runnable 是实际要运行的内容，应该自己在内部保存状态。
	runnable      Runnable
	contextual    *Contextual
	remainingTime uint
}

// Run 包装并运行 Thread 的 runnable。
// 该函数返回的 done、cancel 让 runnable 变得可控：
// - 当 runnable 返回，即 Thread 结束时，done 会接收到 Thread 所属的 Pid 的 string。
// - 当外部需要强制终止 runnable 的运行（调度），调用 cancel() 即可。
func (t *Thread) Run() (done chan string, cancel context.CancelFunc) {
	done = make(chan string)

	_ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-_ctx.Done():
			fmt.Println("Done")
			return
		default:
			t.runnable(t.contextual)
			// finished:
			done <- t.contextual.Process.Id
		}
	}()

	return done, cancel
}

// Process 进程：一个可运行（其中的 Thread 可以运行），集合了资源的东西。
// 为了简化设计，一个 Process 只能持有且必须持有一个 Thread。
type Process struct {
	Id string
	// Precedence 优先级，数字越大越优先
	Precedence uint
	Thread     *Thread
	Memory     Memory
	Devices    map[string]*Device
}

// TODO: Contextual.Commit: after a time_cost (an operation): remainingTime--, schedule.

// Contextual 上下文：线程的上下文。
// 其实就是包含一个指向 Process 的指针。
// 后面还可以往这里加东西：用来保存各种值。
type Contextual struct {
	Process *Process
	// 通过 Contextual.OS.XX 调系统调用
	OS OSInterface
}

// Noop 是一个基本的进程，运行时会使用 fmt.Println 打印 "no-op"。
// 这个东西不需要 IO 设备，不需要内存。
// 运行需要的时间是 0，优先级为最低 (0)。
var Noop = Process{
	Id:         "no-op",
	Precedence: 0,
	Thread: &Thread{
		runnable: func(contextual *Contextual) {
			fmt.Println("no-op")
		},
		contextual:    &Contextual{},
		remainingTime: 0,
	},
	Memory:  Memory{},
	Devices: map[string]*Device{},
}

func init() {
	Noop.Thread.contextual.Process = &Noop
}
