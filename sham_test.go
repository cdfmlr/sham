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

func TestPipe(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.ReadyProcs = []*Process{} // No Noop

	shamOS.CreateProcess("processSend", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			chanNewPipeArg := make(chan interface{}, 2)
			chanNewPipeArg <- "pipe_test" // pipeId
			chanNewPipeArg <- 3           // pipeBufferSize
			contextual.OS.InterruptRequest(contextual.Process.Thread, NewPipeInterrupt, chanNewPipeArg)
			return StatusRunning
		case 1:
			pipe, ok := contextual.Process.Devices["pipe_test"]
			if !ok {
				log.Error("got no pipe!")
				return StatusDone
			}

			log.WithFields(log.Fields{
				"pipe": pipe.GetId(),
			}).Debug("processSend: pipe created successfully")

			if mem.Content == nil {
				mem.Content = map[string]uint{"bpc": 0} // bpc 是下面的 default case 的独立程序计数器
			}
			return StatusRunning
		default:
			bpc := mem.Content.(map[string]uint)["bpc"]
			pipe := interface{}(contextual.Process.Devices["pipe_test"]).(*Pipe)
			switch bpc {
			case 0:
				if pipe.Inputable() {
					pipe.Input() <- "Hello"
					mem.Content.(map[string]uint)["bpc"] += 1
					log.Debug("processSend sent 1/2")
				}
				return StatusRunning
			case 1:
				if pipe.Inputable() {
					pipe.Input() <- "World"
					mem.Content.(map[string]uint)["bpc"] += 1
					log.Debug("processSend sent 2/2")
				}
				return StatusRunning
			}
			log.Debug("processSend finish")
			return StatusDone
		}
	})

	shamOS.CreateProcess("processRecv", 10, 1, func(contextual *Contextual) int {
		mem := &contextual.Process.Memory[0]

		switch contextual.PC {
		case 0:
			chanGetPipeArg := make(chan interface{}, 2)
			chanGetPipeArg <- "pipe_test" // pipeId
			contextual.OS.InterruptRequest(contextual.Process.Thread, GetPipeInterrupt, chanGetPipeArg)
			return StatusRunning
		case 1:
			if pipe, ok := contextual.Process.Devices["pipe_test"]; ok {
				log.WithFields(log.Fields{
					"pipe": pipe.GetId(),
				}).Debug("processRecv: got pipe successfully")

				return StatusRunning
			} else {
				log.Error("got no pipe!")
				return StatusDone
			}
		default:
			if mem.Content == nil {
				mem.Content = map[string]interface{}{"bpc": 0} // bpc 这个 default case 的独立程序计数器
			}

			bpc := mem.Content.(map[string]interface{})["bpc"].(int)
			pipe := interface{}(contextual.Process.Devices["pipe_test"]).(*Pipe)

			switch bpc {
			case 0:
				if pipe.Outputable() {
					mem.Content.(map[string]interface{})["content0"] = <-pipe.Output()
					mem.Content.(map[string]interface{})["bpc"] = bpc + 1
					log.Debug("processSend recv 1/2")
				}
				return StatusRunning
			case 1:
				if pipe.Outputable() {
					mem.Content.(map[string]interface{})["content1"] = <-pipe.Output()
					mem.Content.(map[string]interface{})["bpc"] = bpc + 1
					log.Debug("processSend recv 2/2")
				}
				return StatusRunning
			}

			content0 := mem.Content.(map[string]interface{})["content0"]
			content1 := mem.Content.(map[string]interface{})["content1"]

			log.WithFields(log.Fields{
				"content0": content0,
				"content1": content1,
			}).Debug("processRecv: get content")

			chOutput := make(chan interface{}, 2)
			chOutput <- content0
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chOutput)
			chOutput <- content1
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chOutput)
		}
		return StatusDone
	})

	shamOS.Boot()
}

func TestVarPool(t *testing.T) {
	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.ReadyProcs = []*Process{} // No Noop

	shamOS.CreateProcess("processVarPool", 10, 1, func(contextual *Contextual) int {
		switch {
		case contextual.PC == 0:
			contextual.InitVarPool()

			contextual.SetVar("chOutput", make(chan interface{}, 1))

			log.Debug("VarPool Setup")
			return StatusRunning
		case contextual.PC <= 3:
			contextual.SetVar("num", contextual.PC*contextual.PC)

			chOut := contextual.GetVar("chOutput").(chan interface{})
			chOut <- fmt.Sprintln(contextual.GetVar("num"), chOut)
			contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chOut)

			return StatusRunning
		}

		return StatusDone
	})

	shamOS.Boot()
}

func TestProducerConsumer(t *testing.T) {
	//log.SetLevel(log.ErrorLevel)  // 只看标准输出

	shamOS := NewOS()
	shamOS.Scheduler = FCFSScheduler{}
	shamOS.ReadyProcs = []*Process{} // No Noop

	const PipeProduct = "pipe_product"

	shamOS.CreateProcess("producer", 10, 100, func(contextual *Contextual) int {
		switch contextual.PC {
		case 0:
			log.Debug("producer (PC 0): VarPool Setup")

			contextual.InitVarPool()
			contextual.SetVar("chOutput", make(chan interface{}, 1))

			return StatusRunning
		case 1:
			log.Debug("producer (PC 1): make the product pipe")

			pipeArgs := make(chan interface{}, 2)
			pipeArgs <- PipeProduct // pipeId
			pipeArgs <- 3           // pipeBufferSize

			contextual.OS.InterruptRequest(contextual.Process.Thread, NewPipeInterrupt, pipeArgs)

			return StatusRunning
		default:
			if contextual.PC > 30 {
				chOut := contextual.GetVar("chOutput").(chan interface{})
				chOut <- "producer contextual.PC > 30, stop and exit"
				contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chOut)

				return StatusDone
			}

			_, ok := contextual.TryGetVar("dpc")
			if !ok {
				contextual.SetVar("dpc", 0)
			}
			dpc := contextual.GetVar("dpc").(int)

			switch dpc {
			case 0:
				product := contextual.PC

				contextual.SetVar("product", product)

				log.WithFields(log.Fields{
					"product": product,
				}).Debug("producer produce")

				contextual.SetVar("dpc", 1)
				return StatusRunning
			default:
				pipe := interface{}(contextual.Process.Devices[PipeProduct]).(*Pipe)

				if pipe.Inputable() {
					product := contextual.GetVar("product")

					log.WithField("product", product).Debug("producer put product into PipeProduct")

					pipe.Input() <- product

					contextual.SetVar("dpc", 0)
				} else {
					log.WithFields(log.Fields{
						"PC": contextual.PC,
					}).Debug("producer waiting for consuming")
					return StatusReady // yield
				}
			}

			return StatusRunning
		}
	})

	shamOS.CreateProcess("consumer", 10, 100, func(contextual *Contextual) int {
		switch contextual.PC {
		case 0:
			contextual.InitVarPool()
			contextual.SetVar("chOutput", make(chan interface{}, 1))

			log.Debug("consumer (PC 0): VarPool Setup")
			return StatusRunning
		case 1:
			log.Debug("consumer (PC 1): get the product pipe")

			pipeArgs := make(chan interface{}, 2)
			pipeArgs <- PipeProduct // pipeId

			contextual.OS.InterruptRequest(contextual.Process.Thread, GetPipeInterrupt, pipeArgs)

			return StatusRunning
		default:
			if contextual.PC > 30 {
				log.Debug("consumer exit")
				chOut := contextual.GetVar("chOutput").(chan interface{})
				chOut <- "consumer contextual.PC > 30, stop and exit"
				return StatusDone
			}

			_, ok := contextual.TryGetVar("dpc")
			if !ok {
				contextual.SetVar("dpc", 0)
			}
			dpc := contextual.GetVar("dpc").(int)

			switch dpc {
			case 0:
				pipe := interface{}(contextual.Process.Devices[PipeProduct]).(*Pipe)

				if pipe.Outputable() {
					contextual.SetVar("product", <-pipe.Output())

					contextual.SetVar("dpc", 1)

					log.Debug("consumer get product")
				} else {
					log.WithFields(log.Fields{
						"PC": contextual.PC,
					}).Debug("consumer waiting for product")
					return StatusReady // yield
				}

			default:
				product := contextual.GetVar("product")

				log.WithField("product", product).Debug("consumer consume product")

				chOut := contextual.GetVar("chOutput").(chan interface{})
				chOut <- fmt.Sprintln("consumer consume product:", product)
				contextual.OS.InterruptRequest(contextual.Process.Thread, StdOutInterrupt, chOut)

				contextual.SetVar("dpc", 0)
			}

			return StatusRunning
		}
	})

	shamOS.Boot()
}
