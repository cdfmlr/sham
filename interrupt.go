package sham

import log "github.com/sirupsen/logrus"

// Interrupt 是代表中断的对象
type Interrupt struct {
	Typ     string
	Handler InterruptHandler
	Data    InterruptData
}

// InterruptData 是中断的数据
type InterruptData struct {
	Pid string
	// Channel 是用来给中断发起程序与中断处理程序通信的信道。有两种使用场景：
	// - Output：中断发起者把需要的信息放到 Channel 中，中断处理程序从中取数据；
	// - Input： 或者中断发起者建立一个 Channel，等中断处理程序往里面放东西（注意要把 chan 放到 mem 中，下一个周期时从 mem 中取）；
	// 要同时 IO 的也可以（不推荐），注意用 buffer，自己控制 send&recv 顺序。
	// ⚠️ 这个通信不同步，所以这个 chan 一定要带 buffer，否则会死锁，务必注意！！！！
	Channel chan interface{}
}

// InterruptHandler 是「中断处理程序」
type InterruptHandler func(os *OS, data InterruptData)

// 所有支持的中断类型
const (
	ClockInterrupt       = "ClockInterrupt"
	StdOutInterrupt      = "StdOutInterrupt"
	StdInInterrupt       = "StdInInterrupt"
	NewPipeInterrupt     = "NewPipeInterrupt"
	GetPipeInterrupt     = "GetPipeInterrupt"
	DestroyPipeInterrupt = "DestroyPipeInterrupt"
)

// 中断类型与中断处理程序的映射
var interrupts = map[string]InterruptHandler{
	ClockInterrupt:       HandleClockInterrupt,
	StdOutInterrupt:      HandleStdOutInterrupt,
	StdInInterrupt:       HandleStdInInterrupt,
	NewPipeInterrupt:     HandleNewPipeInterrupt,
	GetPipeInterrupt:     HandleGetPipeInterrupt,
	DestroyPipeInterrupt: HandleDestroyPipeInterrupt,
}

// GetInterrupt 获取中断 —— Interrupt 对象
// 操作系统处理中断请求的时候，通过这个工厂来获取中断。
func GetInterrupt(pid string, typ string, channel chan interface{}) Interrupt {
	return Interrupt{
		Typ:     typ,
		Handler: interrupts[typ],
		Data: InterruptData{
			Pid:     pid,
			Channel: channel,
		},
	}
}

// 下面是各种「中断处理程序」，即 InterruptHandler 的具体实现
// 这些「程序」打印的日志前面统一加 [INT] 标签

// HandleClockInterrupt 处理时钟中断：时间片轮转
func HandleClockInterrupt(os *OS, data InterruptData) {
	log.WithField("pid", data.Pid).Info("[INT] Handle ClockInterrupt: make process ready")
	os.BlockedToReady(data.Pid)
}

// HandleStdOutInterrupt 处理标准输出中断：打印从 data.Channel 读取数据打印到标准输出
func HandleStdOutInterrupt(os *OS, data InterruptData) {
	log.WithField("pid", data.Pid).Info("[INT] Handle StdOutInterrupt: send data to stdout")
	os.Devs["stdout"].Output() <- <-data.Channel
	os.BlockedToReady(data.Pid)
}

// HandleStdOutInterrupt 处理标准输入中断：从标准输入读取数据放到 data.Channel
func HandleStdInInterrupt(os *OS, data InterruptData) {
	log.WithField("pid", data.Pid).Info("[INT] Handle StdInInterrupt: recv data from stdin")
	//log.WithField("in", data.Channel).WithField("&in", &data.Channel).Debug("HandleStdInInterrupt")
	//a := <- os.Devs["stdin"].Input()
	//log.WithField("a", a).WithField("type(a)", fmt.Sprintf("%T", a)).Debug("a := <- os.Devs[stdin].Input()")
	//data.Channel <- a
	data.Channel <- <-os.Devs["stdin"].Input()
	//log.Debug("sent")
	os.BlockedToReady(data.Pid)
}

// HandleNewPipeInterrupt 新建一个 Pipe 设备，并分配给发起中断的进程
// data.Channel 中应该是 [pipeId, pipeBufferSize]，顺序必须正确
func HandleNewPipeInterrupt(os *OS, data InterruptData) {
	pipeId, ok := (<-data.Channel).(string)
	if !ok {
		log.Error("[INT] Handle NewPipeInterrupt: Arg 0 from data.Channel cannot be used as pipeId")
		return
	}
	pipeBufferSize, ok := (<-data.Channel).(int)
	if !ok {
		log.Error("[INT]  Handle NewPipeInterrupt: Arg 1 from data.Channel cannot be used as pipeBufferSize")
		return
	}

	log.WithFields(log.Fields{
		"pid":        data.Pid,
		"pipeId":     pipeId,
		"bufferSize": pipeBufferSize,
	}).Info("[INT] Handle NewPipeInterrupt: create a new Pipe device")

	pipe := NewPipe(pipeId, pipeBufferSize)
	os.Devs[pipeId] = pipe

	if p := os.FindProcess(data.Pid); p != nil {
		p.Devices[pipeId] = pipe
	}

	os.BlockedToReady(data.Pid)
}

// HandleGetPipeInterrupt 获取一个 Pipe 设备，分配给发起中断的进程
// data.Channel 中应该是 pipeId
func HandleGetPipeInterrupt(os *OS, data InterruptData) {
	pipeId, ok := (<-data.Channel).(string)
	if !ok {
		log.Error("[INT] Handle GetPipeInterrupt: Arg 0 from data.Channel cannot be used as pipeId")
		return
	}
	pipe, ok := os.Devs[pipeId]
	if !ok {
		log.WithField("pipeId", pipeId).Error("[INT] Handle GetPipeInterrupt: no such pipe device")
		return
	}

	if proc := os.FindProcess(data.Pid); proc != nil {
		log.WithFields(log.Fields{
			"proc": proc.Id,
			"pipe": pipe,
		}).Info("[INT] Handle GetPipeInterrupt: success")

		proc.Devices[pipeId] = pipe
	}

	os.BlockedToReady(data.Pid)
}

// HandleDestroyPipeInterrupt 将一个 Pipe 设备从 os.Devs 中移除
// data.Channel 中应该是 pipeId
func HandleDestroyPipeInterrupt(os *OS, data InterruptData) {
	pipeId, ok := (<-data.Channel).(string)
	if !ok {
		log.Error("[INT] Handle DestroyPipeInterrupt: Arg 0 from data.Channel cannot be used as pipeId")
		return
	}

	log.WithFields(log.Fields{
		"pipeId": pipeId,
	}).Info("[INT] Handle DestroyPipeInterrupt")

	delete(os.Devs, pipeId)

	os.BlockedToReady(data.Pid)
}
