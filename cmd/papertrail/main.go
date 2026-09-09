package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"papertrail/internal/converter"
	"papertrail/internal/intelligence"
	intelligencepb "papertrail/internal/intelligence/proto"
	"papertrail/internal/metadata"
	"papertrail/internal/pipeline"
	"papertrail/internal/relationship"
	"papertrail/internal/search"
	"papertrail/internal/storage"
	"papertrail/internal/ui"
	"papertrail/logger"
)

func main() {
	log := logger.NewLogger()

	if len(os.Args) < 2 {
		log.Error("usage: papertrail <folder>")
		return
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	log.Info("Starting Python intelligence server")

	pythonServer, err := intelligence.StartPythonServer(ctx)
	if err != nil {
		log.Error("failed to start Python intelligence server: %v", err)
		return
	}
	defer func() {
		if err := pythonServer.Stop(); err != nil {
			log.Error("failed to stop Python intelligence server: %v", err)
		}
	}()

	log.Info("Python intelligence server ready")

	conn, err := grpc.NewClient(
		"127.0.0.1:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Error("failed to connect to intelligence server: %v", err)
		return
	}
	defer conn.Close()

	intelligenceClient := intelligencepb.NewIntelligenceServiceClient(conn)
	metadataEngine := metadata.NewGRPCMetadataEngine(intelligenceClient)

	sqliteStorage, err := storage.NewSQLiteStorage("index.db")
	if err != nil {
		log.Error("failed to initialize storage: %v", err)
		return
	}
	defer sqliteStorage.Close()

	relationshipEngine := relationship.NewEngine(sqliteStorage)

	p := pipeline.New(
		converter.Default(),
		metadataEngine,
		sqliteStorage,
		relationshipEngine,
		log,
		5,
	)

	go func() {
		if err := p.Run(ctx, []string{os.Args[1]}, 2*time.Minute); err != nil {
			log.Error("pipeline stopped: %v", err)
		}
	}()

	searchService := search.NewService(sqliteStorage)
	ui.New(searchService, ctx, cancel).Run()
}
