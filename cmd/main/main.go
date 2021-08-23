package main

import (
	"fmt"
	"os"

	"github.com/trondhumbor/musparser/internal/musparser"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: musparser infile outfile")
		return
	}
	musparser.MusToMidi(os.Args[1], os.Args[2])
}
