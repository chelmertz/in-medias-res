package partille

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"golang.org/x/net/html"
)

type Poller struct{}

func NewPoller() *Poller {
	return &Poller{}
}

func (p *Poller) Close() {}

// PollPartilleBibliotek returns a book from the Partille Bibliotek,
// which you should match against the book you're looking for, probably
// coming from Goodreads
func PollPartilleBibliotek(q BookQuery) (*PollResult, error) {
	now := time.Now().UTC()
	defer func() {
		after := time.Now().UTC()
		fmt.Printf("PollPartilleBibliotek took %s\n", after.Sub(now))
	}()

	searchUrl := fmt.Sprintf(
		"https://bibliotekskatalog.partille.se/cgi-bin/koha/opac-search.pl?advsearch=1&idx=au%%2Cwrdl&q=%s&op=AND&idx=ti&q=%s&weight_search=on&sort_by=popularity_dsc&do=Search",
		url.QueryEscape(q.Author),
		url.QueryEscape(q.Title))
	fmt.Println("got this as a query", searchUrl)

	// let's assume our result is on the first page, or not there at all
	req, err := http.NewRequest(http.MethodGet, searchUrl, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0")
	if err != nil {
		return nil, fmt.Errorf("pollpartillebibliotek: failed to construct request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pollpartillebibliotek: failed to get search page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte("could not read body")
		}
		return nil, fmt.Errorf("pollpartillebibliotek: got status code %d and body %s", resp.StatusCode, body)
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pollpartillebibliotek: failed to parse html: %w", err)
	}

	book := firstResult(node)
	if book == nil {
		return nil, fmt.Errorf("no book could be parsed from %s", searchUrl)
	}

	return book, nil
}

var _ BookAvailabilityPoller = PollPartilleBibliotek // assert the type

var trailingTitleScrap = regexp.MustCompile(`(\s|/)+$`)

func firstResult(node *html.Node) *PollResult {
	book := &PollResult{}
	foundOne := false

	// thanks Partille library for not using a SPA <3
	var traverseDom func(*html.Node)
	traverseDom = func(n *html.Node) {
		isElementWithTitle := false
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && attr.Val == "title" {
					// .title elements represents books
					foundOne = true
					isElementWithTitle = true
					book.Title = n.FirstChild.Data
					book.Title = trailingTitleScrap.ReplaceAllString(book.Title, "")
				} else if attr.Key == "href" {
					book.Url = attr.Val
					// the detail url is relative in the DOM
					book.Url = "https://bibliotekskatalog.partille.se" + book.Url
				}
			}
			if !isElementWithTitle {
				book.Url = ""
			}
		}

		// continue traversing
		for c := n.FirstChild; c != nil && !foundOne; c = c.NextSibling {
			traverseDom(c)
		}
	}

	traverseDom(node)
	if !foundOne {
		return nil
	}

	return book
}
