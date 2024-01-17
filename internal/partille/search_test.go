package partille

import (
	"os"
	"testing"

	"golang.org/x/net/html"
)

func TestCanParseResult(t *testing.T) {
	file, err := os.Open("search_result_example.html")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	node, err := html.Parse(file)
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	result := firstResult(node)
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	expectedTitle := "Den stora sömnen"
	if result.Title != expectedTitle {
		t.Errorf("expected title to be '%s', got '%s'", expectedTitle, result.Title)
	}

	expectedUrl := "https://bibliotekskatalog.partille.se/cgi-bin/koha/opac-detail.pl?biblionumber=10330"
	if result.DetailUrl != expectedUrl {
		t.Errorf("expected url to be %s, got %s", expectedUrl, result.DetailUrl)
	}
}
