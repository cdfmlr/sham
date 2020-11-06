package sham

import (
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

// OS 是模拟的「操作系统」，一个持有并管理 CPU，内存、IO 设备的东西。
// 单核，支持多道程序。
type OS struct {
	CPU  CPU
	Mem  Memory
	Devs map[string]Device

	ProcsMutex   sync.RWMutex
	RunningProc  *Process
	ReadyProcs   []*Process
	BlockedProcs []*Process
	Scheduler    Scheduler

	Interrupts []Interrupt
}

// NewOS 构建一个「操作系统」。
// 新的操作系统有自己控制的 CPU、内存、IO 设备，
// 包含一个 Noop 的进程表以及默认的 NoScheduler 调度器。
func NewOS() *OS {
	return &OS{
		CPU: CPU{},
		Mem: Memory{},
		Devs: map[string]Device{
			"stdout": NewStdOut(),
			"stdin":  NewStdIn(),
		},
		ReadyProcs:   []*Process{&Noop},
		BlockedProcs: []*Process{},
		Scheduler:    NoScheduler{},
		Interrupts:   []Interrupt{},
	}
}

// Boot 启动操作系统。即启动操作系统的调度器。
// 调度器退出标志着操作系统的退出，也就是关机。
func (os *OS) Boot() {
	field := "[OS] "

	log.Info(field, "OS Boot: start scheduler")

	os.Scheduler.schedule(os)

	log.Info(field, "No process to run. Showdown OS.")
}

// HandleInterrupts 处理中断队列中的中断
func (os *OS) HandleInterrupts() {
	var i Interrupt
	for len(os.Interrupts) > 0 {
		i, os.Interrupts = os.Interrupts[0], os.Interrupts[1:]

		log.WithFields(log.Fields{
			"type": i.Typ,
			"data": i.Data,
		}).Info("[OS] Handle Interrupt")

		i.Handler(os, i.Data)
		os.clockTick()
	}
}

/********* 👇 SYSTEM CALLS 👇 ***************/

// OSInterface 是操作系统暴露出来的「系统调用」接口
type OSInterface interface {
	CreateProcess(pid string, precedence uint, timeCost uint, runnable Runnable)
	InterruptRequest(thread *Thread, typ string, channel chan interface{})

	// 这个只是模拟的内部需要，不是真正意义上的系统调用。
	clockTick()
}

// CreateProcess 创建一个进程，放到进程表里
func (os *OS) CreateProcess(pid string, precedence uint, timeCost uint, runnable Runnable) {

	// process
	p := Process{
		Id:         pid,
		Precedence: precedence,
		Devices:    map[string]*Device{},
	}

	// init mem
	// give new process a var table
	os.Mem = append(os.Mem, Object{
		Pid:     pid,
		Content: nil,
	})

	p.Memory = os.Mem[len(os.Mem)-1:]

	// thread
	p.Thread = &Thread{
		runnable: runnable,
		contextual: &Contextual{
			Process: &p,
			OS:      os,
		},
		remainingTime: timeCost,
	}

	// append to ReadyProcs
	os.ReadyProcs = append(os.ReadyProcs, &p)
}

/// InterruptRequest 发出中断请求，阻塞当前进程
func (os *OS) InterruptRequest(thread *Thread, typ string, channel chan interface{}) {
	log.WithFields(log.Fields{
		"thread":  thread,
		"type":    typ,
		"channel": channel,
	}).Info("[OS] InterruptRequest")
	i := GetInterrupt(thread.contextual.Process.Id, typ, channel)
	os.Interrupts = append(os.Interrupts, i)
	os.CPU.Cancel(StatusBlocked)
}

// clockTick 时钟增长
// 这里模拟需要，所以是软的实现，而不是真的"硬件"时钟。
func (os *OS) clockTick() {
	os.CPU.Clock += 1
	time.Sleep(time.Second)
	if os.CPU.Clock%10 == 0 && os.RunningProc.Status == StatusRunning { // 时钟中断
		ch := make(chan interface{}, 1) // buffer 很重要！
		os.InterruptRequest(os.RunningProc.Thread, ClockInterrupt, ch)
		ch <- os.RunningProc
		os.CPU.Clock = 0
	}
}

/********* 👆 SYSTEM CALLS 👆 ***************/

/********* 👇 进程状态转换 👇 ***************/

// RunningToBlocked 阻塞当前运行的进程
func (os *OS) RunningToBlocked() {
	os.ProcsMutex.Lock()
	defer os.ProcsMutex.Unlock()

	log.WithField("process", os.RunningProc).Info("[OS] RunningToBlocked")
	os.RunningProc.Status = StatusBlocked
	os.BlockedProcs = append(os.BlockedProcs, os.RunningProc)

	os.CPU.Unlock()
}

// RunningToReady 把当前运行的进程变成就绪，并释放 CPU
func (os *OS) RunningToReady() {
	os.ProcsMutex.Lock()
	defer os.ProcsMutex.Unlock()

	log.WithField("process", os.RunningProc).Info("[OS] RunningToReady")
	os.RunningProc.Status = StatusReady
	os.ReadyProcs = append(os.ReadyProcs, os.RunningProc)

	os.CPU.Unlock()
}

// RunningToDone 把当前运行的进程标示成完成，并释放 CPU
func (os *OS) RunningToDone() {
	os.ProcsMutex.Lock()
	defer os.ProcsMutex.Unlock()

	log.WithField("process", os.RunningProc).Info("[OS] RunningToDone")
	os.RunningProc.Status = StatusDone

	os.CPU.Unlock()
}

// ReadyToRunning 把就绪队列中的 pid 进程变成运行状态呀
// 这个方法会引导 CPU 切换运行进程，并锁上 CPU
func (os *OS) ReadyToRunning(pid string) {
	os.ProcsMutex.Lock()
	defer os.ProcsMutex.Unlock()

	key := -1
	for i, p := range os.ReadyProcs {
		if p.Id == pid {
			key = i
		}
	}
	log.WithField("process", os.ReadyProcs[key]).Info("[OS] ReadyToRunning")

	os.ReadyProcs[key].Status = StatusRunning
	os.RunningProc = os.ReadyProcs[key]
	os.ReadyProcs = append(os.ReadyProcs[:key], os.ReadyProcs[key+1:]...) // 从就绪队列里删除

	os.CPU.Lock()

	os.CPU.Clock = 0 // 重置时钟计数

	os.CPU.Switch(os.RunningProc.Thread)
}

// BlockedToReady 把阻塞中的 pid 进程变为就绪状态
func (os *OS) BlockedToReady(pid string) {
	os.ProcsMutex.Lock()
	defer os.ProcsMutex.Unlock()

	key := -1
	for i, p := range os.BlockedProcs {
		if p.Id == pid {
			key = i
		}
	}

	if key == -1 {
		log.WithField("pid", pid).Warn("[OS] BlockedToReady Failed: No such Blocked Process")
		return
	}
	log.WithField("process", os.BlockedProcs[key]).Info("[OS] BlockedToReady")

	os.BlockedProcs[key].Status = StatusReady

	os.ReadyProcs = append(os.ReadyProcs, os.BlockedProcs[key])                 // append BlockedProcs[key] into ReadyProcs
	os.BlockedProcs = append(os.BlockedProcs[:key], os.BlockedProcs[key+1:]...) // Delete BlockedProcs[key]
}

/********* 👆 进程状态转换 👆 ***************/
