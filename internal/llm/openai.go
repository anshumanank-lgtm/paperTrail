package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

type Client struct {
	client *openai.Client
	model  openai.ChatModel
}

func New() *Client {
	client := openai.NewClient()

	return &Client{
		client: &client,
		model:  openai.ChatModelGPT5_4Nano,
	}
}

func (c *Client) Answer(
	ctx context.Context,
	question string,
	chunks []string,
) (string, error) {
	if strings.TrimSpace(question) == "" {
		return "", fmt.Errorf("question is empty")
	}

	if len(chunks) == 0 {
		return "", fmt.Errorf("no relevant chunks found")
	}

	var prompt strings.Builder

	prompt.WriteString(
		"Answer the user's question using only the provided document context.\n\n",
	)

	prompt.WriteString("Document context:\n\n")

	for i, chunk := range chunks {
		fmt.Fprintf(
			&prompt,
			"[Chunk %d]\n%s\n\n",
			i+1,
			chunk,
		)
	}

	prompt.WriteString("Question:\n")
	prompt.WriteString(question)

	response, err := c.client.Responses.New(
		ctx,
		responses.ResponseNewParams{
			Model: c.model,
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(prompt.String()),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("openai response: %w", err)
	}

	return response.OutputText(), nil
}
