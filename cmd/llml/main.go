package main

import (
	"fmt"
	"os"

	"github.com/flyingnobita/llml/internal/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
