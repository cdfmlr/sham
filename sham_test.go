package sham

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestNoSchedulerNoop(t *testing.T) {
	shamOS := NewOS()
	shamOS.Boot()
}

func TestFCFSScheduler(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.ReadyProcs = []*Process{&Noop, &Noop}

	log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("before CreateProcess")
	shamOS.CreateProcess("processFoo", 10, 1, func(contextual *Contextual) int {
		for i := 0; i < 3; i++ {
			fmt.Printf("%d From processFoo\n", i)
		}

		// test use mem

		log.WithField("OS.Mem", shamOS.Mem).Debug("before using mem")
		mem := &contextual.Process.Memory[0]
		if mem.Content == nil {
			mem.Content = map[string]string{"hello": "world"}
		}
		log.WithField("OS.Mem", shamOS.Mem).Debug("after using mem")

		// test create new process

		log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("before CreateProcess")
		// A system call!
		contextual.OS.CreateProcess("ProcessBar", 10, 0, func(contextual *Contextual) int {
			fmt.Println("From ProcessBar, a Process dynamic created by processFoo")
			return StatusDone
		})
		log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("after CreateProcess")

		return StatusDone
	})
	log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("after CreateProcess")

	shamOS.Boot()
}

func TestBlock(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.ReadyProcs = []*Process{&Noop, &Noop}

	log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("before CreateProcess")
	shamOS.CreateProcess("processFoo", 10, 1, func(contextual *Contextual) int {
		for i := 0; i < 3; i++ {
			fmt.Printf("%d From processFoo\n", i)
			shamOS.RunningToBlocked()
			log.WithField("BlockedProcs", shamOS.BlockedProcs).Debug("Blocked")
			go func() {
				time.Sleep(2 * time.Second)
				shamOS.BlockedToReady("processFoo")
			}()
		}

		// test use mem

		log.WithField("OS.Mem", shamOS.Mem).Debug("before using mem")
		mem := &contextual.Process.Memory[0]
		if mem.Content == nil {
			mem.Content = map[string]string{"hello": "world"}
		}
		log.WithField("OS.Mem", shamOS.Mem).Debug("after using mem")

		// test create new process

		log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("before CreateProcess")
		// A system call!
		contextual.OS.CreateProcess("ProcessBar", 10, 0, func(contextual *Contextual) int {
			fmt.Println("From ProcessBar, a Process dynamic created by processFoo")
			return StatusDone
		})
		log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("after CreateProcess")
		return StatusDone
	})
	log.WithField("OS.ReadyProcs", shamOS.ReadyProcs).Debug("after CreateProcess")

	shamOS.Boot()
}

func TestCommit(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.CreateProcess("processFoo", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]
		if mem.Content == nil {
			mem.Content = map[string]int{"power": 1}
		}

		logger := log.WithField("mem", mem)

		// 3 clock cost: 0, 1, 2
		for i := 0; i < 3; i++ {
			logger.Debug("[processFoo]")
			mem.Content.(map[string]int)["power"] <<= 1
			contextual.Commit()
		}

		// part_3:
		logger.Debug("part_3")
		fmt.Println("processFoo PC (3 expected):", contextual.PC)
		logger.Debug("exit: StatusDone")
		return StatusDone
	})

	shamOS.Boot()
}

func TestReturnStatus(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.CreateProcess("processFoo", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]
		switch contextual.PC {
		case 0:
			if mem.Content == nil {
				mem.Content = map[string]uint{"PC": contextual.PC}
			}
		case 3:
			fmt.Println("processFoo: PC == 3, exit")
			return StatusDone
		default:
			mem.Content.(map[string]uint)["PC"] += 1
		}
		fooPC := contextual.PC
		contextual.OS.CreateProcess("ProcessBar", 10, 0, func(contextual *Contextual) int {
			fmt.Println("From ProcessBar, a Process dynamic created by processFoo. Parent PC:", fooPC)
			return StatusDone
		})
		return StatusReady
	})

	shamOS.Boot()
}
