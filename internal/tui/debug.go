package tui

import (
	"log"
	"os"
	"strings"
	"sync"
)

const envLLMLDebug = "LLML_DEBUG"

var debugLogger = log.New(os.Stderr, "llml-debug: ", log.LstdFlags|log.Lmicroseconds)

// debugEnabled reads LLML_DEBUG once. debugf is called on every scan step, so
// re-reading the environment each time would be pure overhead, and the value
// cannot change usefully mid-run anyway.
var debugEnabled = sync.OnceValue(func() bool {
	v := strings.TrimSpace(os.Getenv(envLLMLDebug))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
})

func debugf(format string, args ...any) {
	if !debugEnabled() {
		return
	}
	debugLogger.Printf(format, args...)
}
