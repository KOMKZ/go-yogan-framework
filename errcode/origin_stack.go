package errcode

import (
	"fmt"
	"runtime"
	"strings"
)

type originStackProvider interface {
	OriginStack() string
}

func errorOriginStack(cause error) string {
	if provider, ok := cause.(originStackProvider); ok {
		if stack := provider.OriginStack(); stack != "" {
			return stack
		}
	}

	pcs := make([]uintptr, 16)
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var builder strings.Builder
	for index := 0; index < 8; index++ {
		frame, more := frames.Next()
		if frame.Function == "" {
			break
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		_, _ = fmt.Fprintf(&builder, "%s\n\t%s:%d", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return builder.String()
}
