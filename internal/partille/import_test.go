package partille

import (
	"encoding/csv"
	"os"
	"testing"
)

func Test_WhenImported_WillContainUnreadItems(t *testing.T) {
	file, err := os.Open("calle_example.csv")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	reader := csv.NewReader(file)

	s, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory db: %v", err)
	}

	s.ImportGoodreadsCsv(reader)

	// let's abuse that we're in the same package, so
	// we don't need to export test specific methods
	row := s.db.QueryRow("select count(*) from goodread_books")
	if row.Err() != nil {
		t.Fatalf("failed to query db: %v", err)
	}

	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to scan row: %v", err)
	}

	if count != 297 {
		t.Errorf("expected 123 unread books, got %d", count)
	}
}
