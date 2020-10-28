package sham

import (
	log "github.com/sirupsen/logrus"
)

// OS 是模拟的「操作系统」，一个持有并管理 CPU，内存、IO 设备的东西。
// 单核，支持多道程序。
type OS struct {
	CPU  CPU
	Mem  Memory
	Devs map[string]Device

	Procs     []Process
	Scheduler Scheduler
}

// NewOS 构建一个「操作系统」。
// 新的操作系统有自己控制的 CPU、内存、IO 设备，
// 包含一个 Noop 的进程表以及默认的 NoScheduler 调度器。
func NewOS() *OS {
	return &OS{
		CPU:       CPU{},
		Mem:       Memory{},
		Devs:      map[string]Device{},
		Procs:     []Process{Noop},
		Scheduler: NoScheduler{},
	}
}

// Run 启动操作系统。即启动操作系统的调度器。
// 调度器退出标志着操作系统的退出，也就是关机。
func (os *OS) Run() {
	field := "[OS] "

	log.Info(field, "OS Run: start scheduler")
	//fmt.Println("OS Run: start scheduler.")
	os.Scheduler.schedule(&os.CPU, &os.Procs)

	log.Info(field, "scheduler exit. Showdown OS")
}

/********* 👇 SYSTEM CALLS 👇 ***************/

// TODO: interrupt: a set of system-call-like stuffs

// OSInterface 是操作系统暴露出来的「系统调用」接口
type OSInterface interface {
	CreateProcess(pid string, precedence uint, timeCost uint, runnable Runnable)
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

	// append to Procs
	os.Procs = append(os.Procs, p)
}

/********* 👆 SYSTEM CALLS 👆 ***************/
