package sham

import (
	log "github.com/sirupsen/logrus"
	"time"
)

// Scheduler 是模拟的调度器
type Scheduler interface {
	// schedule 完成调度：传入CPU 实例、进程表，
	// 原址完成进程队列调整，并在需要的时候做 CPU.Switch（也可以不做）
	schedule(os *OS)
}

// NoScheduler 是一个不调度的调度器，
// 它只运行表中第一个东西，然后结束退出。
type NoScheduler struct{}

func (n NoScheduler) schedule(os *OS) {
	// 起手式：运行一个线程
	if len(os.ReadyProcs) > 0 {
		os.ReadyToRunning(os.ReadyProcs[0].Id)
	}
	// 调度过程
	for len(os.ReadyProcs) > 0 || os.RunningProc.Status != StatusDone {
		select {
		case pid := <-os.CPU.Done:
			os.RunningToDone()
			log.WithField("done_process", pid).Info("first thread done, do no more schedule. Shutdown NoScheduler")
			return
		}
	}
}

// FCFSScheduler do first-come first-served schedule.
// 先来先服务，一次跑到够(退出｜阻塞)，后来排队尾。
type FCFSScheduler struct{}

func (F FCFSScheduler) schedule(os *OS) {
	field := "[FCFSScheduler] "

	log.Info(field, "FCFSScheduler on")
	if len(os.ReadyProcs) > 0 {
		log.WithField("first_process", os.ReadyProcs[0].Id).Info(field, "Boot the first process")
		os.ReadyToRunning(os.ReadyProcs[0].Id)
	}
	for {
		select {
		case status := <-os.CPU.Done:
			logger := log.WithFields(log.Fields{
				"process":       os.RunningProc.Id,
				"status":        status,
				"contextual_PC": os.RunningProc.Thread.contextual.PC,
			})
			logger.Info(field, "process stop running. Do schedule")
			switch status {
			case StatusDone:
				os.RunningToDone()
			case StatusBlocked:
				os.RunningToBlocked()
			default:
				os.RunningToReady()
			}

			os.HandleInterrupts()

			if len(os.ReadyProcs) > 0 {
				F._schedule(os)
			}
		case <-time.After(3 * time.Second):
			if os.RunningProc.Status != StatusRunning {
				// 避免 "all goroutines are asleep - deadlock"：别闲着，去跑 Noop
				log.Warn("no process ready. Waiting with noop...")

				os.ProcsMutex.Lock()
				os.ReadyProcs = append(os.ReadyProcs, &Noop)
				os.ProcsMutex.Unlock()

				F._schedule(os)
			}
		}

		os.ProcsMutex.RLock()
		hasJobsToDo := len(os.ReadyProcs) > 0 || os.RunningProc.Status != StatusDone || len(os.BlockedProcs) > 0
		os.ProcsMutex.RUnlock()

		if !hasJobsToDo {
			break
		}
	}
	log.Info(field, "All process done. no process to schedule. Shutdown FCFSScheduler")
}

// _schedule 完成真正的调度工作：决定并运行谁
// 该函数假设 process 不为空，且 cpu 空闲（Thread == nil）
func (F FCFSScheduler) _schedule(os *OS) {
	log.WithField("process_to_run", os.ReadyProcs[0].Id).Info("[FCFSScheduler] ", "run the head process")
	os.ReadyToRunning(os.ReadyProcs[0].Id)
}
