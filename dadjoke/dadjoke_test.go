package dadjoke_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/dadjoke-cli/dadjoke"
)

const fakeJokeJSON = `{"id":"abc123","joke":"You know that cemetery up the road? People are dying to get in there.","status":200}`

const fakeSearchJSON = `{"current_page":1,"limit":2,"next_page":2,"previous_page":1,"results":[{"id":"abc123","joke":"My cat was just sick on the carpet."},{"id":"def456","joke":"Why don't cats play poker in the jungle? Too many cheetahs."}],"search_term":"cat","status":200,"total_jokes":11,"total_pages":6}`

func newTestClient(ts *httptest.Server) *dadjoke.Client {
	cfg := dadjoke.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return dadjoke.NewClient(cfg)
}

func TestRandomSendsAcceptJSON(t *testing.T) {
	var gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = fmt.Fprint(w, fakeJokeJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestRandomParsesJoke(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeJokeJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	joke, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if joke.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", joke.ID)
	}
	if joke.Joke != "You know that cemetery up the road? People are dying to get in there." {
		t.Errorf("Joke = %q, unexpected", joke.Joke)
	}
}

func TestSearchParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "cat", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != "abc123" {
		t.Errorf("items[0].ID = %q, want abc123", items[0].ID)
	}
	if items[1].ID != "def456" {
		t.Errorf("items[1].ID = %q, want def456", items[1].ID)
	}
}

func TestSearchSendsPageParam(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "cat", 5, 2)
	if err != nil {
		t.Fatal(err)
	}
	// verify page=2 and limit=5 are in the query
	if gotURL == "" {
		t.Fatal("no query string sent")
	}
	for _, want := range []string{"page=2", "limit=5", "term=cat"} {
		found := false
		for _, part := range splitQuery(gotURL) {
			if part == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("query %q missing %q", gotURL, want)
		}
	}
}

func TestSearchLimitCappedAt30(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		_, _ = fmt.Fprint(w, fakeSearchJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "cat", 99, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range splitQuery(gotURL) {
		if part == "limit=99" {
			t.Errorf("limit=99 sent but should have been capped at 30")
		}
	}
}

func TestRandomRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeJokeJSON)
	}))
	defer ts.Close()

	cfg := dadjoke.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := dadjoke.NewClient(cfg)

	_, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

// splitQuery splits "a=1&b=2" into ["a=1", "b=2"].
func splitQuery(q string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '&' {
			parts = append(parts, q[start:i])
			start = i + 1
		}
	}
	parts = append(parts, q[start:])
	return parts
}
