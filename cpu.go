package sham

import (
	"context"
	"sync"
)

// CPU 处理器：是一个模拟的「CPU」。
// CPU 在某一时刻只能跑一个东西，所以有个锁
// 其中的 Thread 指向正在运行的「线程」
type CPU struct {
	Id string
	sync.Mutex
	Thread *Thread

	Done    chan int
	Blocked chan string
	cancel  context.CancelFunc

	Clock uint
}

// Run 让 CPU 运行任务
func (c *CPU) Run() {
	c.Done, c.cancel = c.Thread.Run()
}

// Cancel 取消 CPU 当前的任务
func (c *CPU) Cancel(status int) {
	if c.cancel != nil {
		if c.Thread.contextual.Process.Status == StatusRunning {
			c.Thread.contextual.Process.Status = status
		}
		c.cancel()
	}
	c.Thread = nil
	c.Done = nil
	c.cancel = nil
}

// Switch 切换 CPU 任务
func (c *CPU) Switch(newThread *Thread) {
	c.Cancel(StatusReady)
	c.Thread = newThread
	c.Run()
}
