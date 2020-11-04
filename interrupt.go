package sham

type Interrupt struct {
	Typ     string
	Handler func(os *OS, data InterruptData)
	Data    InterruptData
}

type InterruptData struct {
	Pid  string
	Data interface{}
}

const (
	ClockInterrupt = "ClockInterrupt"
)

var interrupts = map[string]func(os *OS, data InterruptData){
	ClockInterrupt: HandleClockInterrupt,
}

func GetInterrupt(pid string, typ string, data interface{}) Interrupt {
	return Interrupt{
		Typ:     typ,
		Handler: interrupts[typ],
		Data: InterruptData{
			Pid:  pid,
			Data: data,
		},
	}
}
