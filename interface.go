package ottscanner

import "fmt"

type ILogger interface {
	Debug(value ...any)
	Debugf(message string, value ...any)
	Info(value ...any)
	Infof(message string, value ...any)
}

type TestingLogger struct {
	enabled bool
}

func (tl *TestingLogger) Debug(value ...any) {
	if tl.enabled {
		fmt.Println(value...)
		fmt.Println()
	}
}

func (tl *TestingLogger) Debugf(message string, value ...any) {
	if tl.enabled {
		fmt.Printf(message, value...)
		fmt.Println()
	}
}

func (tl *TestingLogger) Info(value ...any) {
	if tl.enabled {
		fmt.Println(value...)
		fmt.Println()
	}
}

func (tl *TestingLogger) Infof(message string, value ...any) {
	if tl.enabled {
		fmt.Printf(message, value...)
		fmt.Println()
	}
}

func NewTestingLogger(enabled bool) ILogger {
	return &TestingLogger{
		enabled: enabled,
	}
}
