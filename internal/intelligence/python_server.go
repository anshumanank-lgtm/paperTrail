package intelligence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	intelligencepb "papertrail/internal/intelligence/proto"
)

const (
	serverAddress = "127.0.0.1:50051"
	pythonPath    = ".venv/bin/python"
	serverScript  = "internal/intelligence/server.py"
)

type PythonServer struct {
	cmd *exec.Cmd
}

func StartPythonServer(ctx context.Context) (*PythonServer, error) {
	cmd := exec.Command(
		pythonPath,
		serverScript,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"start Python intelligence server: %w",
			err,
		)
	}

	server := &PythonServer{
		cmd: cmd,
	}

	if err := server.waitForReady(ctx); err != nil {
		_ = server.Stop()
		return nil, err
	}

	return server, nil
}

func (s *PythonServer) waitForReady(ctx context.Context) error {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-deadline.C:
			return fmt.Errorf(
				"timeout waiting for Python intelligence server",
			)

		case <-ticker.C:
			if s.isReady(ctx) {
				return nil
			}
		}
	}
}

func (s *PythonServer) isReady(ctx context.Context) bool {
	conn, err := grpc.NewClient(
		serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return false
	}
	defer conn.Close()

	client := intelligencepb.NewIntelligenceServiceClient(conn)

	rpcCtx, cancel := context.WithTimeout(
		ctx,
		500*time.Millisecond,
	)
	defer cancel()

	response, err := client.Health(
		rpcCtx,
		&intelligencepb.HealthRequest{},
	)
	if err != nil {
		return false
	}

	return response.GetReady()
}

func (s *PythonServer) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	// Ask Python to terminate gracefully.
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if err == os.ErrProcessDone {
			return nil
		}

		return fmt.Errorf(
			"send SIGTERM to Python server: %w",
			err,
		)
	}

	done := make(chan error, 1)

	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err

	case <-time.After(5 * time.Second):
		// Graceful shutdown failed. Force termination.
		if err := s.cmd.Process.Kill(); err != nil {
			return fmt.Errorf(
				"kill Python server: %w",
				err,
			)
		}

		return <-done
	}
}
