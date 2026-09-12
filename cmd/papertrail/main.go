package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"papertrail/internal/cli"
	"papertrail/internal/converter"
	"papertrail/internal/intelligence"
	intelligencepb "papertrail/internal/intelligence/proto"
	"papertrail/internal/llm"
	"papertrail/internal/metadata"
	"papertrail/internal/pipeline"
	"papertrail/internal/relationship"
	"papertrail/internal/search"
	"papertrail/internal/storage"
	"papertrail/logger"
)

// main is the entry point of the PaperTrail application. It initializes and starts the backend services,
// including the intelligence server, metadata engine, storage, and search service.
// It also handles command-line interface (CLI) execution if specified.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--cli" {
		runCLI()
		return
	}

	runBackend()
}

// runCLI initializes and runs the command-line interface (CLI) for the PaperTrail application.
func runCLI() {
	commandLine := cli.New(
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)

	commandLine.Run()
}

// runBackend initializes and runs the backend services for the PaperTrail application,
// including the intelligence server, metadata engine, storage, and search service.
// It also handles graceful shutdown on receiving termination signals.
func runBackend() {
	log := logger.NewLogger()

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	log.Info("Starting Python intelligence server")

	pythonServer, err := intelligence.StartPythonServer(ctx)
	if err != nil {
		log.Error(
			"failed to start Python intelligence server: %v",
			err,
		)
		return
	}

	defer func() {
		if err := pythonServer.Stop(); err != nil {
			log.Error(
				"failed to stop Python intelligence server: %v",
				err,
			)
		}
	}()

	log.Info("Python intelligence server ready")

	conn, err := grpc.NewClient(
		"127.0.0.1:50051",
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		log.Error(
			"failed to connect to intelligence server: %v",
			err,
		)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close connection: %v\n", err)
		}
	}()

	intelligenceClient :=
		intelligencepb.NewIntelligenceServiceClient(conn)

	metadataEngine :=
		metadata.NewGRPCMetadataEngine(intelligenceClient)

	sqliteStorage, err :=
		storage.NewSQLiteStorage("index.db")
	if err != nil {
		log.Error(
			"failed to initialize storage: %v",
			err,
		)
		return
	}
	defer func() {
		if err := sqliteStorage.Close(); err != nil {
			log.Error("failed to close SQLite storage: %v", err)
		}
	}()

	relationshipEngine :=
		relationship.NewEngine(sqliteStorage)

	p := pipeline.New(
		converter.Default(),
		metadataEngine,
		sqliteStorage,
		relationshipEngine,
		log,
		5,
	)

	p.Start(ctx)

	llmClient := llm.New()

	searchService :=
		search.NewService(search.StorageRepository{SQLiteStorage: sqliteStorage}, metadataEngine, llmClient)

	controller := cli.NewController(
		ctx,
		p,
		searchService,
		cancel,
	)

	// Start the CLI controller in a separate goroutine to handle user input and commands.
	go func() {
		if err := controller.Start(); err != nil {
			log.Error(
				"failed to start CLI controller: %v",
				err,
			)
			cancel()
		}
	}()

	if err := cli.OpenTerminal(); err != nil {
		log.Error(
			"failed to open CLI terminal: %v",
			err,
		)
		cancel()
		return
	}

	log.Info("PaperTrail backend running")
	log.Info("CLI opened in a new terminal")

	<-ctx.Done()

	log.Info("PaperTrail shutting down")
	p.Wait()
}
