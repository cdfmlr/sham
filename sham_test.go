package sham

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestNoSchedulerNoop(t *testing.T) {
	shamOS := NewOS()
	shamOS.Run()
}

func TestFCFSScheduler(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.Procs = []Process{Noop, Noop}

	shamOS.CreateProcess("processFoo", 10, 1, func(contextual *Contextual) {
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

		log.WithField("OS.Procs", shamOS.Procs).Debug("before CreateProcess")
		// A system call！
		shamOS.CreateProcess("ProcessBar", 10, 0, func(contextual *Contextual) {
			fmt.Println("From ProcessBar, a Process dynamic created by processFoo")
		})
		log.WithField("OS.Procs", shamOS.Procs).Debug("after CreateProcess")
	})

	shamOS.Run()
}
