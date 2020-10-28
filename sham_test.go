package sham

import (
	"testing"
)

func TestNoSchedulerNoop(t *testing.T) {
	shamOS := NewOS()
	shamOS.Run()
}

func TestFCFSScheduler(t *testing.T) {
	shamOS := NewOS()

	shamOS.Scheduler = FCFSScheduler{}

	shamOS.Procs = []Process{Noop, Noop, Noop}

	shamOS.Run()
}
