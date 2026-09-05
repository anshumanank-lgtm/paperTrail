package main

import (
	"os"

	"papertrail/internal/converter"
	"papertrail/internal/metadata"
	"papertrail/internal/pipeline"
	"papertrail/logger"
)

func main() {
	log := logger.NewLogger()

	p := pipeline.New(
		converter.Default(),
		metadata.DefaultMetadataEngine(),
		log,
		4,
	)

	if len(os.Args) < 2 {
		log.Error("usage: papertrail <folder>")
		os.Exit(1)
	}

	if err := p.Run(os.Args[1]); err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}
}
