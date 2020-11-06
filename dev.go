package sham

import (
	"bufio"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
)

// Device 是模拟的「IO设备」的接口
// 调用其 Input() 或 Output() 方法获取通信用的 chan
type Device interface {
	Input() chan interface{}
	Output() chan interface{}
}

// device 是模拟的「IO设备」
// Inspired by *nix /dev
// 某一时刻只能有一个人在用，所以有锁
// 这个模拟里用 channel Input、Output 进行 IO
type device struct {
	Id string
	sync.Mutex

	input  chan interface{}
	output chan interface{}
}

// Input 获取设备的输入信道
func (d *device) Input() chan interface{} {
	log.WithField("device", d.Id).Info("[Device] get input")
	return d.input
}

// Output 获取设备的输出信道
func (d *device) Output() chan interface{} {
	log.WithField("device", d.Id).Info("[Device] get output")
	return d.output
}

// StdOut 是标准输出设备
// 这东西就是把 Output chan 拿到的东西都 fmt.Println 打印出来
// 其实就是一个「生产者-消费者」问题中的「消费者」
type StdOut struct {
	device
}

// StdOutBufferSize：StdOut 的 Output chan 的 buffer 大小
const StdOutBufferSize = 16

// NewStdOut 新建 StdOut 设备，并使其开始工作
func NewStdOut() *StdOut {
	s := &StdOut{}

	s.Id = "stdout"
	s.output = make(chan interface{}, StdOutBufferSize)

	go func() {
		for v := range s.output {
			fmt.Println("<STDOUT>", v)
		}
	}()

	return s
}

// StdIn 是标准输入设备
// 这东西从文件 ./stdin 中逐行读取内容（注意不是 /dev/stdin）
// 放到自己的 Input chan 中。
// 如果文件读完了，就会不停用空字符串""填充 Input chan
// 其实就是一个「生产者-消费者」问题中的「生产者」
type StdIn struct {
	device
}

// StdInBufferSize：StdIn 的 Input chan 的 buffer 大小
// StdIn 会饿汉读取，这里设置 1 可以最大程度节省空间
const StdInBufferSize = 1

// NewStdOut 新建 StdIn 设备，并使其开始工作
func NewStdIn() *StdIn {
	s := &StdIn{}

	s.Id = "stdin"
	s.input = make(chan interface{}, StdInBufferSize)

	go func() {
		// 是这样的，由于这个项目主要通过在 sham_test.go 中写"单元测试"函数来看效果，
		// 而使用 go test 时 os.Stdin 是被定义为 /dev/null 的, see https://groups.google.com/g/golang-nuts/c/k__FvI8nW7Q
		// 我也试过手动打开 /dev/stdin，但并不可以。这样就没法在运行时停下来去拿标准输入了
		// 作为代替，引入一个文件 ./stdin，将要输入的东西写进去，这里回去读这个文件。
		stdin, err := os.Open("./stdin")
		if err != nil {
			log.WithError(err).Error("StdIn Error: failed to open ./stdin")
		}
		defer stdin.Close()

		scanner := bufio.NewScanner(stdin)

		for scanner.Scan() { // 饿汉读取
			line := scanner.Text()
			s.input <- line
			fmt.Println("<STDIN>", line)
		}
		log.WithField("dveice_id", s.Id).
			Warn("[StdIn] No more lines to read from ./stdin, fill input chan with empty strings")
		for { // 没了，填空字符串""
			s.input <- ""
		}
	}()
	return s
}
