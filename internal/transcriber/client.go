package transcriber

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

// Client calls the Whisper microservice to transcribe audio.
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Minute}, // Whisper can be slow on CPU
	}
}

// Segment is one timestamped chunk of speech from Whisper.
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"` // seconds
	End   float64 `json:"end"`   // seconds
	Text  string  `json:"text"`
}

// Result is the full transcription response from the Whisper service.
type Result struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language"`
	Duration float64   `json:"duration"`
	Elapsed  float64   `json:"elapsed"`
}

// TranscribeBytes uploads raw audio bytes to the Whisper service and returns the transcript.
// filename determines the file extension so Whisper picks the right decoder.
func (c *Client) TranscribeBytes(audio []byte, filename string) (*Result, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	part, err := w.CreateFormFile("audio", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(audio)); err != nil {
		return nil, fmt.Errorf("write audio: %w", err)
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/transcribe", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper error %d: %s", resp.StatusCode, raw)
	}

	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}

// Ping checks if the Whisper service is reachable.
func (c *Client) Ping() error {
	resp, err := c.http.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("whisper not reachable at %s: %w", c.baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whisper health check failed: %d", resp.StatusCode)
	}
	return nil
}
