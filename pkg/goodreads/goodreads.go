package goodreads

import (
	"context"
	"encoding/json"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	"golang.org/x/xerrors"
	"net/http"
	"net/url"
)

// An API is used to query good reads.
type API struct {
	key     string
	limiter *rate.Limiter
}

// New creates a new API
func New(key string) *API {
	return &API{
		key: key,
		// the goodreads terms of service requires us not to make more than 1 request per second.
		limiter: rate.NewLimiter(1, 1),
	}
}

// BookReviewsByTitle searches for book reviews by title.
func (api *API) BookReviewsByTitle(ctx context.Context, title, author string) error {
	log.Info().
		Str("title", title).
		Str("author", author).
		Msg("searching for book reviews by title")
	var res interface{}
	err := api.get(ctx, "book/title.json", url.Values{
		"title":  {title},
		"author": {author},
	}, &res)
	if err != nil {
		return err
	}
	log.Info().Interface("response", res).Send()
	return nil
}

func (api *API) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	params.Set("key", api.key)
	req, err := http.NewRequest("GET", "https://www.goodreads.com/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return xerrors.Errorf("error generating request: %w", err)
	}
	req = req.WithContext(ctx)
	return api.do(req, out)
}

func (api *API) do(req *http.Request, out interface{}) error {
	req.Header.Set("Accept", "application/json")

	req, ht := nethttp.TraceRequest(opentracing.GlobalTracer(), req)
	defer ht.Finish()

	err := api.limiter.Wait(req.Context())
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	if res.StatusCode/100 != 2 {
		return xerrors.Errorf("unexpected status code from goodreads (%d): %s",
			res.StatusCode,
			res.Status)
	}

	err = json.NewDecoder(res.Body).Decode(out)
	if err != nil {
		return xerrors.Errorf("error decoding goodreads response as json: %w", err)
	}

	return nil
}
