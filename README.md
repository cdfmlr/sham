# Sham

> A sham (simulated) machine for OS learning..

这个小项目实现了一个模拟“计算机”与“操作系统”。

这个项目使用在 Go 语言，抽象并实现了 CPU、内存、IO设备、操作系统、运行在操作系统上的进程、线程，以及几个具体的应用程序。

## How it works

### 抽象结构

这个模拟的操作系统中，主要包含以下抽象：

- CPU：一个带有互斥锁的结构，单独在一个协程中运行应用程序的线程。
- 内存：一个无限大的结构，不需要管理。
- IO设备：一个独立的设备，单独在一个协程中运行，可以输入或输出。
- 操作系统：包括操作系统基础结构、调度器、系统调用、中断处理程序组。为例简化模型，操作系统单独运行在一个协程中，它本身不需要在 CPU、内存上运行，而是在内部持有 CPU 和内存。
  - 操作系统基础结构：持有并管理 CPU，内存、IO 设备、进程表和中断向量。
  - 调度器：完成进程调度工作的具体算法。
  - 系统调用：为应用程序提供的“内核态”操作接口。
  - 中断处理程序：处理应用程序或操作系统内部发出的各类中断。
- 进程、线程：为了简化模型，一个进程只能持有且必须持有一个线程。
  - 进程：包括一个可运行的进程、当前状态（运行、就绪、阻塞）以及需要的各种资源（内存等）。
  - 线程：进程具体可执行的部分，包含要运行的“程序代码”以及运行时的上下文。
  - 上下文：包括程序计数器、进程指针、操作系统接口。
- 具体应用程序：进程的实例（在 [sham_test.go](./sham_test.go) 中实现了一些有意思的应用）。

目前的实现中，调度器使用的是一个带时间片轮转的 FCFS 调度器，但可以十分容易地添加、替换新的调度算法。

### 系统的工作流程

<details>
<summary>系统整体工作的流程:</summary>

```flow
bootOS=>start: 操作系统启动
shutdownOS=>end: 操作系统关机
startScheduler=>operation: 启动调度器
shutdownScheduler=>operation: 关闭调度器
condHasProcess=>condition: 系统中有进程
handleINT=>subroutine: 操作系统处理中断向量
schedule=>operation: 调度器调整就绪队列,
选择一个进程运行
runProcess=>subroutine: 运行进程
processDone=>operation: 进程结束/阻塞/时间片用尽

bootOS->startScheduler->condHasProcess
condHasProcess(yes)->handleINT->schedule->runProcess->processDone(left)->condHasProcess
condHasProcess(no)->shutdownScheduler->shutdownOS
```

</details>

<details>
<summary>进程运行流程:</summary>

```flow
schedulerCallRun=>start: 调度器选择进程运行
returnScheduler=>end: 结束
osChangeStatus=>operation: 操作系统修改进程的状态为“运行”
getCPU=>operation: 获取 CPU 锁，将线程交给 CPU
cpuRun=>operation: CPU 运行线程
threadRun=>subroutine: 线程运行，直到结束，或被阻塞
schedulerCallRun->osChangeStatus->getCPU->cpuRun->threadRun->returnScheduler
```

</details>

![截屏2020-11-13 19.58.42](https://tva1.sinaimg.cn/large/0081Kckwly1gknstu9g88j31e00u0dkx.jpg)

### 线程的运行

关于线程的运行，直接看一部分 Thread 具体代码实现（有删减修改）：

```go
// Thread 线程：是一个可以在 CPU 里跑的东西。
type Thread struct {
	// runnable 是实际要运行的内容
	runnable Runnable
	// contextual 是 Thread 的环境
	contextual *Contextual
}

// Runnable 程序：应用程序的具体的代码就写在这里面
// 每一次返回就代表“一条指令”（一个原子操作）执行完毕，返回值为状态：
//  - StatusRunning 继续运行（如果时间片未用尽）
//  - StatusReady 会进入就绪队列（即 yield，主动让出 CPU）
//  - StatusBlocked 会进入阻塞状态（一般不用。需要阻塞时一般通过中断请求）
//  - StatusDone 进程运行结束。
type Runnable func(contextual *Contextual) int

// 进程的状态
const (
	StatusBlocked = -1
	StatusReady   = 0
	StatusRunning = 1
	StatusDone    = 2
)

// Contextual 上下文：线程的上下文。
type Contextual struct {
    // 指向 Process 的指针，可以取用 Process 的资源
	Process *Process
	// 通过 Contextual.OS.XX 调系统调用
	OS OSInterface
	// 程序计数器
	PC uint
}
```

这就是简要的线程结构。具体的应用程序把“代码”写在一个 Runnable 型的函数中，这个函数每执行完一个原子操作就应该返回一次。同时 runnable 也可以在执行完任何一个原子操作时，被外部事件强行打断（比如时间片用尽）。  `Thread.Run()`  实现了这个控制，runnable 的返回、外部的打断都会被 `Thread.Run()` 捕获。前面的流程图中，「CPU 运行线程」也就是调用 `Thread.Run()` 方法：

```go
// Run 包装并运行 Thread 的 runnable。
// 该函数返回的 done、cancel 让 runnable 变得可控：
// - 当 runnable 返回，即 Thread 结束时，done 会接收到 Process/Thread 的状态。
// - 当外部需要强制终止 runnable 的运行（调度），调用 cancel() 即可。
func (t *Thread) Run() (done chan int, cancel context.CancelFunc) {
	done = make(chan int)

	_ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for { // 一条条代码不停跑，直到阻塞｜退出｜被取消
			select {
			case <-_ctx.Done(): // 被取消，取消由 CPU 发起
                // 取消时 CPU 会临时置 Status 为需要转到的状态，
				// 这里获取并把这个值传给操作系统
				// 同时把状态重置为 StatusRunning
                //（状态转化需由操作系统完成，这里只是暂时借用这个值，故要还原）
				s := t.contextual.Process.Status
				t.contextual.Process.Status = StatusRunning
				done <- s
				return
			default:
				ret := t.runnable(t.contextual)
                t.contextual.Commit()  // PC++, clockTick()
				if ret != StatusRunning { // 结束|阻塞|就绪，交给调度器处理
					done <- ret
					return
				}
			}
		}
	}()

	return done, cancel
}
```

这里使用 go 语言标准库的 context 包，控制线程运行的取消（外部打断）。如果没有外部打断，runnable 返回时就会将程序计数器和时钟加一。如果一直返回 StatusRunning 就可以一直运行下去，在 runnable 内部，通过 `contextual.PC` 以及流程控制语句即可实现一条条代码执行的效果。

例如，下面的这个 Runnable 函数（程序）有 4 个操作（4 条指令）它们会依次执行：

```go
func processSeq(contextual *Contextual) int {
    switch contextual.PC {
    case 0:
        log.Debug("Line 0")
        return StatusRunning
    case 1:
        log.Debug("Line 1")
        return StatusRunning
    case 2:
        log.Debug("Line 2")
        return StatusRunning
    case 3:
        return StatusDone
    }
    return StatusDone
}
```

这样的写法有些像汇编语言，而事实上，runnable 甚至就是这个模拟的操作系统的机器语言！所以写起来略为繁琐，这个难以避免。

把上面的程序翻译成我们平时的 C/C++ 程序就是：

```c
int main() {
    log.Debug("Line 0")
	log.Debug("Line 1")
	log.Debug("Line 2")
    return 0  // 这里使用 C 的传统，而事实上 StatusDone == 2
}
```

运行的效果如下：

![截屏2020-11-13 11.21.54](https://tva1.sinaimg.cn/large/0081Kckwly1gkndrmwyhcj31hm09sn1b.jpg)

### 中断与 IO

中断在操作系统中至关重要，所以这个模拟的操作系统也需要中断的实现。

这里对中断的建模如下：

```go
type Interrupt struct {
	Typ     string
	Handler InterruptHandler
	Data    InterruptData
}

// InterruptHandler 是「中断处理程序」
type InterruptHandler func(os *OS, data InterruptData)

// InterruptData 是中断的数据
type InterruptData struct {
	Pid string
	// Channel 是用来给中断发起程序与中断处理程序通信的信道。
	Channel chan interface{}
}
```

中断包含一个类型，处理程序（处理程序和中断类型一一对应），以及附加的数据。

应用程序，或操作系统内部都可以通过一个**系统调用**发出中断（这个模拟中几乎没有硬件，所以不考虑硬件中断）。操作系统受到中断请求，会把中断放入一个中断表，然后在当前时钟周期结束后，调度运行时程序阻塞，依次调用相应中断处理程序处理中断表里的中断。

中断处理程序会做必要的工作，完成后解除发起进程的阻塞状态。

有了这个中断机制，就可以很容易地实现时间片轮转——在时间片用尽后，发出一个时钟中断，其处理程序会把进程就绪。

同时，中断支持了各种 IO 设备的使用。例如，最简单的是使用标准输出，发出一个标准输出的中断请求，传入需要的数据即可：

```go
func helloWorld(contextual *Contextual) int {
    ch := make(chan interface{}, 1)
    ch <- "Hello, world!"
    contextual.OS.InterruptRequest(contextual.Process.Thread, 
                                   StdOutInterrupt, ch)
    return StatusDone
}
```

这个中断的处理程序会使用标准输出设备，完成输出。运行结果：

![截屏2020-11-13 11.18.57](https://tva1.sinaimg.cn/large/0081Kckwly1gkndndoqc4j31jc0dk7au.jpg)

出了标准输入，我还实现了标准输出设备，以及特殊的 Pipe 设备。这两个东西的调用代码都比较复杂，在此不做演示。

标准输入、输出都是模拟的“硬件”，这些东西会单独在一个协程中运行。而 Pipe 设备则是一个虚拟的设备，它是对 Go 语言 channel 的封装。操作系统可以动态生成任意多个 Pipe 并分配给需要的进程，通过 Pipe 进程之间就可以利用 CSP 模型可以完成通信与同步。

利用 Pipe 设备，在这个模拟的操作系统中，可以实现「生产者-消费者」问题。代码稍微复杂（需要约 150 行代码，见 [sham_test.go](./sham_test.go) 之 TestProducerConsumer），但完全可以正确工作：

![截屏2020-11-13 12.07.16](https://tva1.sinaimg.cn/large/0081Kckwly1gknf17o30tj31jj0u0hbm.jpg)

## TODO

- [ ] More scheduler (different kinks of algorithms)
- [ ] English README and comments
- [ ] More Devices
- [ ] ...

## LICENSE

Copyright 2020 CDFMLR

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

```
   http://www.apache.org/licenses/LICENSE-2.0
```

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.