package sdk

import (
	"fmt"
	"time"
)

func Step(label string, fn func()) {
	ts := time.Now().Format(time.RFC3339)
	logMsg("step", label)
	emitEvent(Event{Type: "step", Step: label, Status: "started", Timestamp: ts})

	defer func() {
		if r := recover(); r != nil {
			failTs := time.Now().Format(time.RFC3339)
			logMsg("step", fmt.Sprintf("%s failed: %v", label, r))
			emitEvent(Event{Type: "step", Step: label, Status: "failed", Timestamp: failTs, Error: fmt.Sprint(r)})
			panic(r)
		}
	}()

	fn()

	doneTs := time.Now().Format(time.RFC3339)
	logMsg("step", label+" done")
	emitEvent(Event{Type: "step", Step: label, Status: "completed", Timestamp: doneTs})
}
