package dadjoke

import (
	"context"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes dadjoke as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/dadjoke-cli/dadjoke"
//
// The same Domain also builds the standalone dadjoke binary.
func init() { kit.Register(Domain{}) }

// Domain is the dadjoke driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "dadjoke",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "dadjoke",
			Short:  "Dad jokes from icanhazdadjoke.com",
			Long: `dadjoke fetches dad jokes from icanhazdadjoke.com.
No API key or authentication required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/dadjoke-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// random: get a random dad joke
	kit.Handle(app, kit.OpMeta{
		Name:    "random",
		Group:   "read",
		List:    false,
		Summary: "Get a random dad joke",
	}, randomOp)

	// search: search for dad jokes by term
	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Search dad jokes by term",
		Args:    []kit.Arg{{Name: "term", Help: "search term"}},
	}, searchOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type randomInput struct {
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type searchInput struct {
	Term   string        `kit:"arg" help:"search term"`
	Limit  int           `kit:"flag,inherit" help:"max results (max 30)"`
	Page   int           `kit:"flag,inherit" help:"page number (1-based)"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

// --- handlers ---

func randomOp(ctx context.Context, in randomInput, emit func(Joke) error) error {
	joke, err := in.Client.Random(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(joke)
}

func searchOp(ctx context.Context, in searchInput, emit func(Joke) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 30 {
		limit = 30
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	items, err := in.Client.Search(ctx, in.Term, limit, page)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty dadjoke reference")
	}
	return "joke", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "joke":
		return "https://icanhazdadjoke.com/j/" + id, nil
	default:
		return "", errs.Usage("dadjoke has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
