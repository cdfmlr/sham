package sham

import (
	log "github.com/sirupsen/logrus"
)

// Scheduler 是模拟的调度器
type Scheduler interface {
	// schedule 完成调度：传入CPU 实例、进程表，
	// 原址完成进程队列调整，并在需要的时候做 CPU.Switch（也可以不做）
	schedule(cpu *CPU, process []Process)

	// add 添加行的进程。
	add(newProcess *Process, process []Process)
}

// NoScheduler 是一个不调度的调度器，
// 它只运行表中第一个东西，然后结束退出。
type NoScheduler struct{}

func (n NoScheduler) schedule(cpu *CPU, process []Process) {
	// 起手式：运行一个线程
	if len(process) > 0 {
		cpu.Switch(process[0].Thread)
	}
	// 调度过程
	for len(process) > 0 {
		select {
		case pid := <-cpu.Done:
			log.WithField("done_process", pid).Info("first thread done, do no more schedule")
			log.Info("first thread done, do no more schedule. Shutdown NoScheduler")
			return
		}
	}
}

func (n NoScheduler) add(newProcess *Process, process []Process) {
	panic("not support.")
}

// FCFSScheduler do first-come first-served schedule.
// 先来先服务，一次跑到够(退出｜阻塞)，后来排队尾。
type FCFSScheduler struct{}

func (F FCFSScheduler) schedule(cpu *CPU, process []Process) {
	field := "[FCFSScheduler] "

	log.Info(field, "FCFSScheduler on")
	if len(process) > 0 {
		log.WithField("first_process", process[0].Id).Info(field, "Run the first process")
		cpu.Switch(process[0].Thread)
	}
	for len(process) > 0 {
		select {
		case pid := <-cpu.Done:
			log.WithField("done_process", pid).Info(field, "process done. Do schedule")
			process = process[1:] // 已完成，移除
			if len(process) > 0 {
				F._schedule(cpu, process)
			}
		}
	}
	log.Info(field, "All process done. no process to schedule. Shutdown FCFSScheduler")
}

// _schedule 完成真正的调度工作：决定并运行谁
// 该函数假设 process 不为空，且 cpu 空闲（Thread == nil）
func (F FCFSScheduler) _schedule(cpu *CPU, process []Process) {
	log.WithField("process_to_run", process[0].Id).Info("[FCFSScheduler] ", "run the head process")
	cpu.Switch(process[0].Thread)
}

func (F FCFSScheduler) add(newProcess *Process, process []Process) {
	process = append(process, *newProcess)
}
