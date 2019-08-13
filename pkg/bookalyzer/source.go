package bookalyzer

import (
	"bufio"
	"context"
	"fmt"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"golang.org/x/xerrors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// BookDetails are the details for a book.
type BookDetails struct {
	Title    string
	Author   string
	Language string
	Content  io.ReadCloser
}

// A Source is an a source for book downloads.
type Source interface {
	Download(ctx context.Context) (*BookDetails, error)
	BookURL() string
	FilePath() string
}

// GetSource returns the source for the url
func GetSource(bookURL string) (Source, error) {
	gs, err := NewGutenbergSource(bookURL)
	if err != nil {
		return nil, err
	}
	return gs, nil
}

// A GutenbergSource reads books from project gutenberg.
type GutenbergSource struct {
	id string
}

// NewGutenbergSource creates a new project gutenberg source.
func NewGutenbergSource(bookURL string) (*GutenbergSource, error) {
	u, err := url.Parse(bookURL)
	if err != nil {
		return nil, err
	}
	if u.Host != "www.gutenberg.org" {
		return nil, xerrors.New("not a gutenberg url")
	}
	if !strings.HasPrefix(u.Path, "/ebooks/") {
		return nil, xerrors.Errorf("invalid gutenberg URL. Should be https://www.gutenberg.org/ebooks/{ID}")
	}
	id := u.Path[len("/ebooks/"):]
	return &GutenbergSource{id: id}, nil
}

// BookURL returns the book URL for a project gutenberg book.
func (gs *GutenbergSource) BookURL() string {
	return fmt.Sprintf("https://www.gutenberg.org/files/%s/%s-0.txt",
		gs.id, gs.id)
}

// FilePath returns the file path for a project gutenberg book.
func (gs *GutenbergSource) FilePath() string {
	return GetFilePath(gs.BookURL())
}

// Download downloads the book.
func (gs *GutenbergSource) Download(ctx context.Context) (*BookDetails, error) {
	req, err := http.NewRequest("GET", gs.BookURL(), nil)
	if err != nil {
		return nil, xerrors.Errorf("error generating download request: %w", err)
	}
	req = req.WithContext(ctx)
	req, ht := nethttp.TraceRequest(opentracing.GlobalTracer(), req)
	defer ht.Finish()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, xerrors.Errorf("error downloading book: %w", err)
	}

	if res.StatusCode/100 != 2 {
		return nil, xerrors.Errorf("invalid http status when downloading book: %s", res.Status)
	}

	details := new(BookDetails)

	scanner := bufio.NewScanner(res.Body)
loop:
	for scanner.Scan() {
		switch {
		case strings.HasPrefix(scanner.Text(), "Title: "):
			details.Title = strings.TrimSpace(scanner.Text()[len("Title: "):])
		case strings.HasPrefix(scanner.Text(), "Author: "):
			details.Author = strings.TrimSpace(scanner.Text()[len("Author: "):])
		case strings.HasPrefix(scanner.Text(), "Language: "):
			details.Language = strings.TrimSpace(scanner.Text()[len("Language: "):])
		case strings.HasPrefix(scanner.Text(), "*** START OF THIS PROJECT GUTENBERG EBOOK"):
			break loop
		}
	}

	pr, pw := io.Pipe()
	go func() {
		defer res.Body.Close()

		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "*** END OF THIS PROJECT GUTENBERG EBOOK") {
				_, _ = io.Copy(ioutil.Discard, res.Body)
				break
			} else {
				_, err := pw.Write(scanner.Bytes())
				if err != nil {
					_ = pw.CloseWithError(err)
					return
				}

				_, err = pw.Write([]byte("\n"))
				if err != nil {
					_ = pw.CloseWithError(err)
					return
				}
			}
		}
		_ = pw.CloseWithError(scanner.Err())
	}()
	details.Content = pr
	return details, nil
}
