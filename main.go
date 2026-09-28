package main

import (
	"log"
	"os"

	kmaker "gitlab.com/seabeam/schematic/platform/services/kmaker/internal/kmaker"
)

func main() {
	if err := kmaker.Run(os.Args[1:]); err != nil {
		log.Fatalf("kmaker: %v", err)
	}
}
