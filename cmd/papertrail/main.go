package main

import (
	"os"

	"papertrail/internal/converter"
	"papertrail/internal/metadata"
	"papertrail/internal/pipeline"
	"papertrail/internal/relationship"
	"papertrail/internal/storage"
	"papertrail/logger"
)

func main() {
	log := logger.NewLogger()

	if len(os.Args) < 2 {
		log.Error("usage: papertrail <folder>")
		os.Exit(1)
	}

	sqliteStorage, err := storage.NewSQLiteStorage("index.db")
	if err != nil {
		log.Error("failed to initialize storage: %v", err)
		os.Exit(1)
	}
	defer sqliteStorage.Close()

	relationshipEngine := relationship.NewEngine(sqliteStorage)

	p := pipeline.New(
		converter.Default(),
		metadata.DefaultMetadataEngine(),
		sqliteStorage,
		relationshipEngine,
		log,
		4,
	)

	if err := p.Run(os.Args[1]); err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}
}
