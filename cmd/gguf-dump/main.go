// Command gguf-dump prints all GGUF metadata key-value pairs (and optionally tensors).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/flyingnobita/llml/internal/models"
)

func main() {
	tensors := flag.Bool("tensors", false, "also list tensor names, types, and shapes (can be very long)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <path.gguf>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	err := models.DumpGGUF(os.Stdout, args[0], models.DumpGGUFOptions{Tensors: *tensors})
	if err != nil {
		fmt.Fprintf(os.Stderr, "gguf-dump: %v\n", err)
		os.Exit(1)
	}
}
