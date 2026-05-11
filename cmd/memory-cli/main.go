package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("memory-cli: use the HTTP API for now. Commands to add: search, trace, export, eval.")
		return
	}
	fmt.Printf("memory-cli command %q is not implemented in the MVP yet\n", os.Args[1])
}
