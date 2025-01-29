package altda

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNotFound is returned when the server could not find the input.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput is returned when the input is not valid for posting to the DA storage.
var ErrInvalidInput = errors.New("invalid input")

// DAClient is an HTTP client to communicate with a DA storage service.
// It creates commitments and retrieves input data + verifies if needed.
type DAClient struct {
	url string
	// verify sets the client to verify a Keccak256 commitment on read.
	verify bool
	// whether commitment is precomputable (only applicable to keccak256)
	precompute bool
	getTimeout time.Duration
	putTimeout time.Duration
}

func NewDAClient(url string, verify bool, pc bool) *DAClient {
	return &DAClient{
		url:        url,
		verify:     verify,
		precompute: pc,
	}
}

// GetInput returns the input data for the given encoded commitment bytes.
func (c *DAClient) GetInput(ctx context.Context, comm CommitmentData) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/get/0x%x", c.url, comm.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	client := &http.Client{Timeout: c.getTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get preimage: %v", resp.StatusCode)
	}
	defer resp.Body.Close()
	input, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.verify {
		if err := comm.Verify(input); err != nil {
			return nil, err
		}

	}
	return input, nil
}

// SetInput sets the input data and returns the respective commitment.
func (c *DAClient) SetInput(ctx context.Context, img []byte) (CommitmentData, error) {
	if len(img) == 0 {
		return nil, ErrInvalidInput
	}

	if c.precompute { // precompute commitment (only applicable to keccak256)
		comm := NewKeccak256Commitment(img)
		if err := c.setInputWithCommit(ctx, comm, img); err != nil {
			return nil, err
		}

		return comm, nil
	}

	// let DA server generate commitment
	return c.setInput(ctx, img)

}

// setInputWithCommit sets a precomputed commitment for some pre-image data.
func (c *DAClient) setInputWithCommit(ctx context.Context, comm CommitmentData, img []byte) error {
	// encode with commitment type prefix
	key := comm.Encode()
	body := bytes.NewReader(img)
	url := fmt.Sprintf("%s/put/0x%x", c.url, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	client := &http.Client{Timeout: c.putTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to store preimage: %v", resp.StatusCode)
	}

	return nil
}

// setInput sets the input data and reads the respective DA generated commitment.
func (c *DAClient) setInput(ctx context.Context, img []byte) (CommitmentData, error) {
	if len(img) == 0 {
		return nil, ErrInvalidInput
	}

	body := bytes.NewReader(img)
	url := fmt.Sprintf("%s/put/", c.url)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	client := &http.Client{Timeout: c.putTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to store data: %v", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	comm, err := DecodeCommitmentData(b)
	if err != nil {
		return nil, err
	}

	return comm, nil
}
