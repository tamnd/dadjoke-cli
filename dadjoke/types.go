package dadjoke

// Joke is one dad joke.
type Joke struct {
	ID   string `kit:"id" json:"id"`
	Joke string `json:"joke"`
}

// unexported: only used inside dadjoke.go for JSON decode

type rawJoke struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

type searchResponse struct {
	Results    []rawJoke `json:"results"`
	TotalJokes int       `json:"total_jokes"`
	SearchTerm string    `json:"search_term"`
}
