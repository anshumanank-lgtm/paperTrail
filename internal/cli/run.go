package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/google/uuid"

	"papertrail/internal/pipeline"
	"papertrail/internal/search"
)

const socketPath = "/tmp/papertrail.sock"

// commandRequest represents a command request sent from the CLI to the backend.
type commandRequest struct {
	Command  string   `json:"command"`
	Path     string   `json:"path,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	ID       string   `json:"id,omitempty"`
	Question string   `json:"question,omitempty"`
}

// commandResponse represents a command response sent from the backend to the CLI.
type commandResponse struct {
	OK      bool     `json:"ok"`
	Done    bool     `json:"done"`
	Message string   `json:"message,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

// Controller handles CLI commands and coordinates actions with the pipeline and search service.
type Controller struct {
	ctx           context.Context
	pipeline      *pipeline.Pipeline
	searchService *search.Service
	shutdown      context.CancelFunc

	foldersMu sync.Mutex
	folders   []string

	scanMu sync.Mutex
}

type readCloser struct {
	io.Reader
}

func (readCloser) Close() error {
	return nil
}

// NewController creates a new Controller instance with the provided context, pipeline, search service, and
// shutdown function.
// It initializes the Controller with the given parameters and returns a pointer to the new instance.
// The Controller is responsible for handling CLI commands and coordinating actions with the pipeline
// and search service.
func NewController(
	ctx context.Context,
	p *pipeline.Pipeline,
	searchService *search.Service,
	shutdown context.CancelFunc,
) *Controller {
	return &Controller{
		ctx:           ctx,
		pipeline:      p,
		searchService: searchService,
		shutdown:      shutdown,
	}
}

// Start starts the CLI controller, listening for incoming connections on a Unix socket.
// It handles incoming requests and executes the corresponding commands.
// The controller runs until the context is canceled or an error occurs while accepting connections.
func (c *Controller) Start() error {
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("CLI socket error: %w", err)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return fmt.Errorf("CLI socket permissions error: %w", err)
	}

	defer func() {
		if err := listener.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close listener: %v\n", err)
		}
	}()
	defer func() {
		if err := os.Remove(socketPath); err != nil {
			fmt.Fprintf(os.Stderr, "remove socket path: %v\n", err)
		}
	}()

	go func() {
		<-c.ctx.Done()
		defer func() {
			if err := listener.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "close listener: %v\n", err)
			}
		}()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if c.ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("CLI accept error: %w", err)
		}

		go c.handle(conn)
	}
}

// handle processes an incoming connection, decoding the command request and executing the corresponding action.
// It sends the response back to the client and closes the connection when done.
func (c *Controller) handle(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close connection: %v\n", err)
		}
	}()

	var request commandRequest

	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		return
	}

	encoder := json.NewEncoder(conn)

	emit := func(message string) {
		_ = encoder.Encode(commandResponse{
			OK:      true,
			Done:    false,
			Message: message,
		})
	}

	response := c.execute(request, emit)
	response.Done = true

	_ = encoder.Encode(response)

	if request.Command == "exit" && c.shutdown != nil {
		c.shutdown()
	}
}

// execute processes the command request and returns the corresponding response.
// It handles various commands such as adding/removing folders, scanning, listing documents,
// retrieving document/entity details, and asking questions.
// The emit function is used to send intermediate messages back to the client during command execution.
func (c *Controller) execute(
	request commandRequest,
	emit func(string),
) commandResponse {
	switch request.Command {
	case "add":
		return c.addFolder(request.Path, emit)

	case "remove":
		return c.removeFolder(request.Path)

	case "folders":
		c.foldersMu.Lock()
		paths := append([]string(nil), c.folders...)
		c.foldersMu.Unlock()

		return commandResponse{
			OK:      true,
			Paths:   paths,
			Message: "Configured folders",
		}

	case "scan":
		return c.scan(emit)

	case "documents":
		return c.listDocuments()

	case "document":
		return c.getDocument(request.ID)

	case "entity":
		return c.getEntity(request.ID)

	case "status":
		c.foldersMu.Lock()
		count := len(c.folders)
		c.foldersMu.Unlock()

		return commandResponse{
			OK:      true,
			Message: fmt.Sprintf("%d folder(s) configured", count),
		}

	case "help":
		return commandResponse{
			OK:      true,
			Message: "Commands: add <folder>, remove <folder>, folders, scan, documents, document <id>, entity <id>, status, help, exit",
		}

	case "exit":
		return commandResponse{
			OK:      true,
			Message: "Bye",
		}

	case "ask":
		return c.ask(request.Question)

	default:
		return commandResponse{
			OK:      false,
			Message: "Unknown command",
		}
	}
}

// addFolder adds a folder to the list of configured folders and initiates a scan of the folder.
// It checks if the folder path is valid and not already configured. If the folder is successfully added,
// it triggers a scan of the folder and returns a response indicating the result.
func (c *Controller) addFolder(
	path string,
	emit func(string),
) commandResponse {
	path = strings.TrimSpace(path)

	if path == "" {
		return commandResponse{
			OK:      false,
			Message: "Folder path required",
		}
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: err.Error(),
		}
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: err.Error(),
		}
	}

	if !info.IsDir() {
		return commandResponse{
			OK:      false,
			Message: "Path is not a directory",
		}
	}

	c.foldersMu.Lock()

	for _, existing := range c.folders {
		if existing == absolute {
			c.foldersMu.Unlock()

			return commandResponse{
				OK:      false,
				Message: "Folder already configured",
			}
		}
	}

	c.folders = append(c.folders, absolute)
	c.foldersMu.Unlock()

	if err := c.runScan(
		[]string{absolute},
		emit,
	); err != nil {
		return commandResponse{
			OK:      false,
			Message: fmt.Sprintf("Scan failed: %v", err),
		}
	}

	return commandResponse{
		OK:      true,
		Message: "Folder added",
	}
}

// removeFolder removes a folder from the list of configured folders.
// It checks if the folder path is valid and exists in the list of configured folders.
func (c *Controller) removeFolder(path string) commandResponse {
	path = strings.TrimSpace(path)

	absolute, err := filepath.Abs(path)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: err.Error(),
		}
	}

	c.foldersMu.Lock()
	defer c.foldersMu.Unlock()

	for i, existing := range c.folders {
		if existing == absolute {
			c.folders = append(
				c.folders[:i],
				c.folders[i+1:]...,
			)

			return commandResponse{
				OK:      true,
				Message: "Folder removed",
			}
		}
	}

	return commandResponse{
		OK:      false,
		Message: "Folder not configured",
	}
}

// scan initiates a scan of the configured folders and returns the result.
func (c *Controller) scan(
	emit func(string),
) commandResponse {
	c.foldersMu.Lock()
	paths := append([]string(nil), c.folders...)
	c.foldersMu.Unlock()

	if len(paths) == 0 {
		return commandResponse{
			OK:      false,
			Message: "No folders configured",
		}
	}

	if err := c.runScan(paths, emit); err != nil {
		return commandResponse{
			OK:      false,
			Message: fmt.Sprintf("Scan failed: %v", err),
		}
	}

	return commandResponse{
		OK: true,
	}
}

// runScan initiates a scan of the specified folder paths and emits messages during the scanning process.
func (c *Controller) runScan(
	paths []string,
	emit func(string),
) error {
	c.scanMu.Lock()
	defer c.scanMu.Unlock()

	return c.pipeline.Scan(
		c.ctx,
		paths,
		emit,
	)
}

// listDocuments retrieves the list of documents from the search service and returns a command response.
func (c *Controller) listDocuments() commandResponse {
	documents, err := c.searchService.ListDocuments(c.ctx)
	if err != nil {
		return commandResponse{
			OK: false,
			Message: fmt.Sprintf(
				"Failed to list documents: %v",
				err,
			),
		}
	}

	if len(documents) == 0 {
		return commandResponse{
			OK:      true,
			Message: "No documents indexed",
		}
	}

	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Documents: %d\n\n",
		len(documents),
	)

	fmt.Fprintf(
		&b,
		"%-38s  %-30s  %-18s  %s\n",
		"ID",
		"TITLE",
		"TYPE",
		"FILENAME",
	)

	b.WriteString(
		"--------------------------------------------------------------------------------------------------------------\n",
	)

	for _, doc := range documents {
		title := strings.TrimSpace(
			doc.DocumentMetadata.Title,
		)
		if title == "" {
			title = "-"
		}

		fmt.Fprintf(
			&b,
			"%-38s  %-30s  %-18s  %s\n",
			doc.ID,
			title,
			doc.DocumentMetadata.DocumentType,
			doc.FileMetadata.Filename,
		)
	}

	return commandResponse{
		OK:      true,
		Message: strings.TrimRight(b.String(), "\n"),
	}
}

// getDocument retrieves the details of a specific document by its ID and returns a command response.
func (c *Controller) getDocument(id string) commandResponse {
	documentID, err := parseUUID(id)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: err.Error(),
		}
	}

	detail, err := c.searchService.GetDocumentDetail(
		c.ctx,
		documentID,
	)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: fmt.Sprintf("Failed to get document: %v", err),
		}
	}

	var b strings.Builder

	title := strings.TrimSpace(
		detail.Document.DocumentMetadata.Title,
	)
	if title == "" {
		title = detail.Document.FileMetadata.Filename
	}

	fmt.Fprintf(&b, "Document: %s\n", title)
	fmt.Fprintf(&b, "ID: %s\n", detail.Document.ID)
	fmt.Fprintf(&b, "Filename: %s\n", detail.Document.FileMetadata.Filename)
	fmt.Fprintf(&b, "Type: %s\n", detail.Document.DocumentMetadata.DocumentType)
	fmt.Fprintf(&b, "Path: %s\n", detail.Document.FileMetadata.SourcePath)
	fmt.Fprintf(&b, "Size: %d\n", detail.Document.FileMetadata.Size)
	fmt.Fprintf(
		&b,
		"Modified: %s\n",
		detail.Document.FileMetadata.ModifiedAt.Format("2006-01-02 15:04:05"),
	)

	b.WriteString("\nGraph:\n")

	fmt.Fprintf(
		&b,
		"[%s]\n",
		title,
	)
	fmt.Fprintf(
		&b,
		"ID: %s\n",
		detail.Document.ID,
	)

	// Entities connected to this document.
	if len(detail.Entities) > 0 {
		b.WriteString("├── Entities\n")

		for i, entity := range detail.Entities {
			prefix := "│   ├──"
			if i == len(detail.Entities)-1 {
				prefix = "│   └──"
			}

			fmt.Fprintf(
				&b,
				"%s %s: %s\n",
				prefix,
				entity.Type,
				entity.Value,
			)

			idPrefix := "│   │   "
			if i == len(detail.Entities)-1 {
				idPrefix = "│       "
			}

			fmt.Fprintf(
				&b,
				"%sID: %s\n",
				idPrefix,
				entity.ID,
			)
		}
	} else {
		b.WriteString("├── Entities: None\n")
	}

	// Documents connected through relationships.
	if len(detail.Relationships) > 0 {
		b.WriteString("└── Relationships\n")

		for i, relationship := range detail.Relationships {
			prefix := "    ├──"
			if i == len(detail.Relationships)-1 {
				prefix = "    └──"
			}

			fmt.Fprintf(
				&b,
				"%s %s\n",
				prefix,
				relationship.Relationship.RelationshipType,
			)

			otherDocument := relationship.OtherDocument

			otherTitle := strings.TrimSpace(
				otherDocument.DocumentMetadata.Title,
			)
			if otherTitle == "" {
				otherTitle = otherDocument.FileMetadata.Filename
			}

			fmt.Fprintf(
				&b,
				"    │   Document: %s\n",
				otherTitle,
			)
			fmt.Fprintf(
				&b,
				"    │   ID: %s\n",
				otherDocument.ID,
			)

			if relationship.Relationship.Confidence != nil {
				fmt.Fprintf(
					&b,
					"    │   Confidence: %.2f\n",
					*relationship.Relationship.Confidence,
				)
			}

			if len(relationship.Relationship.Entities) > 0 {
				b.WriteString("    │   Entities:\n")

				for _, entity := range relationship.Relationship.Entities {
					fmt.Fprintf(
						&b,
						"    │   └── %s: %s\n",
						entity.Type,
						entity.Value,
					)
					fmt.Fprintf(
						&b,
						"    │       ID: %s\n",
						entity.ID,
					)
				}
			}
		}
	} else {
		b.WriteString("└── Relationships: None\n")
	}

	return commandResponse{
		OK:      true,
		Message: strings.TrimRight(b.String(), "\n"),
	}
}

// getEntity retrieves the details of a specific entity by its ID and returns a command response.
func (c *Controller) getEntity(id string) commandResponse {
	entityID, err := parseUUID(id)
	if err != nil {
		return commandResponse{
			OK:      false,
			Message: err.Error(),
		}
	}

	detail, err := c.searchService.GetEntityDetail(
		c.ctx,
		entityID,
	)
	if err != nil {
		return commandResponse{
			OK: false,
			Message: fmt.Sprintf(
				"Failed to get entity: %v",
				err,
			),
		}
	}

	var b strings.Builder

	b.WriteString("Entity\n\n")

	fmt.Fprintf(
		&b,
		"ID: %s\n",
		detail.Entity.ID,
	)
	fmt.Fprintf(
		&b,
		"Type: %s\n",
		detail.Entity.Type,
	)
	fmt.Fprintf(
		&b,
		"Value: %s\n",
		detail.Entity.Value,
	)

	b.WriteString("\nDocuments:\n\n")

	if len(detail.Documents) == 0 {
		b.WriteString("None")
	} else {
		fmt.Fprintf(
			&b,
			"%-38s  %-30s  %-18s  %s\n",
			"ID",
			"TITLE",
			"TYPE",
			"FILENAME",
		)

		b.WriteString(
			"--------------------------------------------------------------------------------------------------------------\n",
		)

		for _, doc := range detail.Documents {
			title := strings.TrimSpace(
				doc.DocumentMetadata.Title,
			)
			if title == "" {
				title = "-"
			}

			fmt.Fprintf(
				&b,
				"%-38s  %-30s  %-18s  %s\n",
				doc.ID,
				title,
				doc.DocumentMetadata.DocumentType,
				doc.FileMetadata.Filename,
			)
		}
	}

	return commandResponse{
		OK:      true,
		Message: strings.TrimRight(b.String(), "\n"),
	}
}

// parseUUID parses a string value into a UUID, returning an error if the value is not a valid UUID.
func parseUUID(value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return uuid.Nil, fmt.Errorf("ID required")
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID: %s", value)
	}

	return id, nil
}

// CLI represents the command-line interface for interacting with the PaperTrail application.
type CLI struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

// New creates a new CLI instance with the provided input, output, and error streams.
// It initializes the CLI with the given parameters and returns a pointer to the new instance.
func New(
	in io.Reader,
	out io.Writer,
	err io.Writer,
) *CLI {
	return &CLI{
		in:  in,
		out: out,
		err: err,
	}
}

// Run starts the CLI, displaying the available commands and handling user input.
func (c *CLI) Run() int {
	fmt.Fprintln(c.out, "PaperTrail CLI")
	fmt.Fprintln(c.out)
	fmt.Fprintln(c.out, "Commands:")
	fmt.Fprintln(c.out, "  add <folder>")
	fmt.Fprintln(c.out, "  remove <folder>")
	fmt.Fprintln(c.out, "  folders")
	fmt.Fprintln(c.out, "  scan")
	fmt.Fprintln(c.out, "  documents")
	fmt.Fprintln(c.out, "  document <id>")
	fmt.Fprintln(c.out, "  entity <id>")
	fmt.Fprintln(c.out, "  ask <question>")
	fmt.Fprintln(c.out, "  status")
	fmt.Fprintln(c.out, "  help")
	fmt.Fprintln(c.out, "  exit")
	fmt.Fprintln(c.out)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "papertrail> ",
		Stdin:           readCloser{c.in},
		Stdout:          c.out,
		Stderr:          c.err,
		HistoryFile:     "",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		if _, werr := fmt.Fprintln(c.err, err); werr != nil {
			_ = werr
		}
		return 1
	}
	defer func() {
		if err := rl.Close(); err != nil {
			_, _ = fmt.Fprintf(c.err, "close readline: %v\n", err)
		}
	}()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF {
				return 0
			}

			if _, werr := fmt.Fprintln(c.err, err); werr != nil {
				_ = werr
			}
			return 1
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		args := strings.Fields(line)

		request := commandRequest{
			Command: args[0],
		}

		switch args[0] {
		case "add", "remove":
			if len(args) < 2 {
				if _, err := fmt.Fprintln(c.out, "Path required"); err != nil {
					_, _ = fmt.Fprintf(c.err, "write prompt output: %v\n", err)
				}
				continue
			}

			request.Path = strings.Join(args[1:], " ")

		case "document", "entity":
			if len(args) != 2 {
				if _, err := fmt.Fprintln(c.out, "ID required"); err != nil {
					_, _ = fmt.Fprintf(c.err, "write prompt output: %v\n", err)
				}
				continue
			}

			request.ID = args[1]

		case "scan", "folders", "documents", "status", "help", "exit":

		case "ask":
			if len(args) < 2 {
				if _, err := fmt.Fprintln(c.out, "Question required"); err != nil {
					_, _ = fmt.Fprintf(c.err, "write prompt output: %v\n", err)
				}
				continue
			}

			request.Question = strings.Join(args[1:], " ")

		default:
			if _, err := fmt.Fprintln(c.out, "Unknown command. Type help."); err != nil {
				_, _ = fmt.Fprintf(c.err, "write prompt output: %v\n", err)
			}
			continue
		}

		if err := sendRequest(
			request,
			func(response commandResponse) {
				if response.Message != "" {
					fmt.Fprintln(c.out, response.Message)
				}

				for _, path := range response.Paths {
					fmt.Fprintln(c.out, path)
				}
			},
		); err != nil {
			fmt.Fprintln(c.err, err)
			continue
		}

		if request.Command == "exit" {
			return 0
		}
	}
}

// sendRequest sends a command request to the backend via a Unix socket and handles the response.
// It establishes a connection to the backend, encodes the request, and decodes the response.
// The onResponse callback is invoked for each response received from the backend.
// If an error occurs during the request or response handling, it returns an error.
func sendRequest(
	request commandRequest,
	onResponse func(commandResponse),
) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf(
			"backend unavailable: %w",
			err,
		)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "close socket connection: %v\n", err)
		}
	}()

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return err
	}

	decoder := json.NewDecoder(conn)

	for {
		var response commandResponse

		if err := decoder.Decode(&response); err != nil {
			return err
		}

		onResponse(response)

		if response.Done {
			if !response.OK {
				return fmt.Errorf("%s", response.Message)
			}

			return nil
		}
	}
}

// ask processes a question using the search service and returns the answer in a command response.
// It validates the question, retrieves the answer from the search service, and handles any errors that may occur.
func (c *Controller) ask(
	question string,
) commandResponse {
	answer, err := c.searchService.Ask(
		c.ctx,
		question,
	)
	if err != nil {
		return commandResponse{
			OK: false,
			Message: fmt.Sprintf(
				"Ask failed: %v",
				err,
			),
		}
	}

	return commandResponse{
		OK:      true,
		Message: answer,
	}
}
