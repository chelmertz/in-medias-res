package partille

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/net/html"
)

func testFile(filename string) *html.Node {
	file, err := os.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to read file: %v", err))
	}

	node, err := html.Parse(file)
	if err != nil {
		panic(fmt.Sprintf("failed to parse html: %v", err))
	}
	return node
}

func TestCanParseMultipleResults(t *testing.T) {
	node := testFile("search_result_four.html")
	result := firstResult(node)
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	expectedTitle := "Den stora sömnen"
	if result.Title != expectedTitle {
		t.Errorf("expected title to be '%s', got '%s'", expectedTitle, result.Title)
	}
}

func TestCanParseSingleResults(t *testing.T) {
	node := testFile("search_result_single.html")
	result := firstResult(node)
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	expectedTitle := "Den stora sömnen"
	if result.Title != expectedTitle {
		t.Errorf("expected title to be '%s', got '%s'", expectedTitle, result.Title)
	}

	if !result.IsAvailable {
		t.Errorf("expected title to be available but it wasn't")
	}
}

func TestCanRepresentEmptyResult(t *testing.T) {
	node := testFile("search_result_empty_example.html")
	result := firstResult(node)
	if result != nil {
		t.Fatalf("expected result, got nil")
	}
}
