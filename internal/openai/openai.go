package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Model string

const (
	GPT4 Model = "gpt-4-1106-preview"
)

type Service struct {
	c *http.Client

	model Model
	key   string
}

func NewService(key string, model Model) Service {
	return Service{
		c: &http.Client{
			Timeout: 30 * time.Second,
		},
		model: model,
		key:   key,
	}
}

// ChatSync blocks until the full response has been returned by GPT.
func (s Service) ChatSync(prompt string) (string, error) {
	rsp, err := s.callCompletions(prompt, false)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	var cr completionsResponse
	if err := json.NewDecoder(rsp.Body).Decode(&cr); err != nil {
		return "", nil
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("chatsync: empty choices list in response")
	}
	return cr.Choices[0].Message.Content, nil
}

type Stream struct {
	err error
	rsp io.ReadCloser
	buf []byte
}

func (s *Stream) Close() error {
	return s.rsp.Close()
}

func (s *Stream) Err() error {
	return s.err
}

func (s *Stream) Done() bool {
	return s.err != nil
}

func (s *Stream) Next() string {
	buf := make([]byte, 2<<10) // 4kb
	n, err := s.rsp.Read(buf)
	if err != nil {
		s.err = err
		return ""
	}

	s.buf = append(s.buf, buf[:n]...)

	var i, count, start int
	for i < len(s.buf) {
		if s.buf[i] == '{' {
			count++
			if count == 1 {
				start = i
			}
		}
		if s.buf[i] == '}' {
			count--
			if count == 0 {
				obj := s.buf[start : i+1]
				s.buf = s.buf[i+1:]
				return parseChunk(obj)
			}
		}
		i++
	}

	return ""
}

func parseChunk(objChunk []byte) string {
	var chunk completionsStreamChunk
	if err := json.Unmarshal(objChunk, &chunk); err != nil {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

// ChatAsync returns as soon as the request is made, and if successful,
// will stream the result to an io.Reader.
func (s Service) ChatAsync(prompt string) (*Stream, error) {
	rsp, err := s.callCompletions(prompt, true)
	if err != nil {
		return nil, err
	}
	return &Stream{rsp: rsp.Body}, nil
}

type (
	completionsRequest struct {
		Stream   bool                 `json:"stream"`
		Model    string               `json:"model"`
		Messages []completionsMessage `json:"messages"`
	}

	completionsResponse struct {
		Choices []completionsChoice `json:"choices"`
	}

	completionsMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	completionsChoice struct {
		Message completionsMessage `json:"message"`
	}

	completionsStreamChunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
)

func (s Service) callCompletions(prompt string, stream bool) (*http.Response, error) {
	req, err := s.createCompletionsRequest(prompt, stream)
	if err != nil {
		return nil, err
	}
	rsp, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode < 200 || rsp.StatusCode > 299 {
		return nil, fmt.Errorf("completions: bad status code %d", rsp.StatusCode)
	}
	return rsp, nil
}

func (s Service) createCompletionsRequest(prompt string, stream bool) (*http.Request, error) {
	body := completionsRequest{
		Stream: stream,
		Model:  string(s.model),
		Messages: []completionsMessage{
			{
				Role:    "assistant",
				Content: "Hi, I'm here to help with your questions.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.openai.com/v1/chat/completions",
		&buf,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.key)

	return req, nil
}
