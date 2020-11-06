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

func TestSeq(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	// 这是一个标准的顺序运行的进程
	shamOS.CreateProcess("processSeq", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			if mem.Content == nil {
				mem.Content = map[string]uint{"count": 0}
			}
			log.Debug("Line 0")
			return StatusRunning
		case 1:
			log.Debug("Line 1")
			mem.Content.(map[string]uint)["count"] += 1
			return StatusRunning
		case 2:
			log.Debug("Line 2")
			mem.Content.(map[string]uint)["count"] += 1
			return StatusRunning
		case 3:
			if mem.Content.(map[string]uint)["count"] == 2 {
				fmt.Println("count == 2, exit")
				return StatusDone
			}
		}
		return StatusDone
	})

	shamOS.Boot()
}

func TestCancel(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	go func() {
		time.Sleep(2 * time.Second)
		shamOS.CPU.Cancel(StatusReady) // if StatusBlocked: all blocked， run noops
	}()

	shamOS.CreateProcess("processSeq", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			if mem.Content == nil {
				mem.Content = map[string]uint{"count": 0}
			}
			log.Debug("Line 0")
			return StatusRunning
		case 1:
			log.Debug("Line 1")
			mem.Content.(map[string]uint)["count"] += 1
			return StatusRunning
		case 2:
			log.Debug("Line 2")
			mem.Content.(map[string]uint)["count"] += 1
			return StatusRunning
		case 3:
			if mem.Content.(map[string]uint)["count"] == 2 {
				fmt.Println("count == 2, exit")
				return StatusDone
			}
		}
		return StatusDone
	})

	shamOS.Boot()
}

func TestClockInterrupt(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.CreateProcess("processSeq", 10, 1, func(contextual *Contextual) int {
		switch {
		case contextual.PC < 30:
			contextual.OS.CreateProcess(fmt.Sprintf("subprocess%d", contextual.PC), 10, 0, func(contextual *Contextual) int {
				fmt.Println(contextual.Process.Id)
				return StatusDone
			})
			log.WithField("PC", contextual.PC).Debug("processSeq continue")
			return StatusRunning
		case contextual.PC == 30:
			log.WithField("PC", contextual.PC).Debug("processSeq exit")
		}
		return StatusDone
	})

	shamOS.Boot()
}

func TestStdOut(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.CreateProcess("processSeq", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]
		chanOutput := make(chan interface{}, 10)

		switch contextual.PC {
		case 0:
			if mem.Content == nil {
				mem.Content = map[string]uint{"count": 0}
			}
			log.Debug("Line 0")
			chanOutput <- mem.Content.(map[string]uint)["count"]
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
			return StatusRunning
		case 1:
			log.Debug("Line 1")
			mem.Content.(map[string]uint)["count"] += 1
			chanOutput <- mem.Content.(map[string]uint)["count"]
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
			return StatusRunning
		case 2:
			log.Debug("Line 2")
			mem.Content.(map[string]uint)["count"] += 1
			chanOutput <- mem.Content.(map[string]uint)["count"]
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
			return StatusRunning
		case 3:
			if mem.Content.(map[string]uint)["count"] == 2 {
				fmt.Println("By fmt.Println: count == 2, exit")
				chanOutput <- "By StdOut: count == 2, exit"
				contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
				return StatusDone
			}
		}
		return StatusDone
	})

	shamOS.Boot()
}

func TestHelloWorld(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.ReadyProcs = []*Process{} // No Noop

	shamOS.CreateProcess("processSeq", 10, 1, func(contextual *Contextual) int {

		ch := make(chan interface{}, 1)
		ch <- "Hello, world!"
		contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, ch)
		return StatusDone
	})

	shamOS.Boot()
}

func TestStdIn(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.ReadyProcs = []*Process{} // No Noop

	shamOS.CreateProcess("processSeq_0", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			in := make(chan interface{}, 1)
			// in 会在多个周期中被使用，需要放入内存
			mem.Content = map[string]chan interface{}{"in": in}

			contextual.OS.InterruptRequest(contextual.Process.Thread, StdInInterrupt, in)
			return StatusRunning
		case 1:
			in := mem.Content.(map[string]chan interface{})["in"]

			log.Debug("to recv")
			content := <-in
			log.WithField("content", content).Debug("got content")
			out := make(chan interface{}, 1)
			out <- content
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, out)
			return StatusDone
		}

		return StatusDone
	})

	shamOS.CreateProcess("processSeq_1", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			in := make(chan interface{}, 2)
			// in 会在多个周期中被使用，需要放入内存
			mem.Content = map[string]chan interface{}{"in": in}

			// 要求多个输入
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdInInterrupt, in)
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdInInterrupt, in)
			return StatusRunning
		case 1:
			in := mem.Content.(map[string]chan interface{})["in"]

			log.Debug("to recv")
			content := (<-in).(string) + (<-in).(string)
			log.WithField("content", content).Debug("got content")
			out := make(chan interface{}, 1)
			out <- content
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, out)
			return StatusDone
		}

		return StatusDone
	})

	//shamOS.ReadyProcs = shamOS.ReadyProcs[:1]

	shamOS.Boot()
}

func TestClockInterruptMeetIO(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}

	shamOS.CreateProcess("processMixItr", 10, 1, func(contextual *Contextual) int {
		chanOutput := make(chan interface{}, 10)

		switch {
		case contextual.PC <= 9:
			log.WithField("PC", contextual.PC).Debug("waiting...")
			return StatusRunning
		case contextual.PC == 10:
			chanOutput <- "output something just before clock interrupt"
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
			return StatusRunning
		case contextual.PC == 11:
			chanOutput <- "output something just after clock interrupt"
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chanOutput)
			return StatusDone
		}
		return StatusDone
	})

	shamOS.Boot()
}
