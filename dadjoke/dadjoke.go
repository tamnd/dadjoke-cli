// Package dadjoke is the library behind the dadjoke command line:
// the HTTP client, request shaping, and the typed data models for the
// dad joke API at icanhazdadjoke.com.
//
// The API is completely open — no authentication or API key required.
// The Client must send Accept: application/json on every request, otherwise
// the API returns HTML. It paces requests and retries transient 429 and
// 5xx failures with exponential backoff.
package dadjoke

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sync"
	"time"
)

const (
	// Host is the site this client talks to.
	Host = "icanhazdadjoke.com"
)

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://icanhazdadjoke.com",
		UserAgent: "Mozilla/5.0 (compatible; dadjoke-cli/dev; +https://github.com/tamnd/dadjoke-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to icanhazdadjoke.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Random returns a random dad joke.
func (c *Client) Random(ctx context.Context) (Joke, error) {
	u := c.cfg.BaseURL + "/"
	body, err := c.get(ctx, u)
	if err != nil {
		return Joke{}, err
	}
	var raw rawJoke
	if err := json.Unmarshal(body, &raw); err != nil {
		return Joke{}, fmt.Errorf("decode joke: %w", err)
	}
	return Joke{ID: raw.ID, Joke: raw.Joke}, nil
}

// Search returns dad jokes matching term. limit is capped at 30; pass 0 to use default 20.
// page is 1-based; pass 0 to use page 1.
func (c *Client) Search(ctx context.Context, term string, limit, page int) ([]Joke, error) {
	n := limit
	if n <= 0 {
		n = 20
	}
	if n > 30 {
		n = 30
	}
	p := page
	if p <= 0 {
		p = 1
	}
	u := fmt.Sprintf("%s/search?term=%s&limit=%d&page=%d", c.cfg.BaseURL, neturl.QueryEscape(term), n, p)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	items := make([]Joke, 0, len(resp.Results))
	for _, r := range resp.Results {
		items = append(items, Joke{ID: r.ID, Joke: r.Joke})
	}
	return items, nil
}

// Get returns the dad joke with the given id.
func (c *Client) Get(ctx context.Context, id string) (Joke, error) {
	u := c.cfg.BaseURL + "/j/" + id
	body, err := c.get(ctx, u)
	if err != nil {
		return Joke{}, err
	}
	var raw rawJoke
	if err := json.Unmarshal(body, &raw); err != nil {
		return Joke{}, fmt.Errorf("decode joke: %w", err)
	}
	return Joke{ID: raw.ID, Joke: raw.Joke}, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	// CRITICAL: must send Accept: application/json or the API returns HTML
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
