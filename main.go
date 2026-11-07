package main

import (
	"log"
	"os"

	kmaker "github.com/tacplus-dev/kmaker/internal/kmaker"
)

func main() {
	if err := kmaker.Run(os.Args[1:]); err != nil {
		log.Fatalf("kmaker: %v", err)
	}
}
