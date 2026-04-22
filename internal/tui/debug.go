package tui

import (
	"log"
	"os"
	"strings"
)

const envLLMLDebug = "LLML_DEBUG"

var debugLogger = log.New(os.Stderr, "llml-debug: ", log.LstdFlags|log.Lmicroseconds)

func debugEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envLLMLDebug))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func debugf(format string, args ...any) {
	if !debugEnabled() {
		return
	}
	debugLogger.Printf(format, args...)
}
