package partille

import (
	"encoding/csv"
	"os"
	"testing"
)

func csvFixture(t *testing.T) *csv.Reader {
	file, err := os.Open("calle_example.csv")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	return csv.NewReader(file)
}

func Test_WhenImported_WillContainUnreadItems(t *testing.T) {
	s, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite memory db: %v", err)
	}
	userId, err := s.CreateUser("margarethe")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	s.ImportGoodreadsCsv(csvFixture(t), userId)

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

	if count != 396 {
		t.Errorf("expected 396 unread books, got %d", count)
	}

	row = s.db.QueryRow("select count(*) from user_goodread_books where user_id = ?", userId)
	if row.Err() != nil {
		t.Fatalf("failed to query db: %v", err)
	}

	var usersBookCount int
	if err := row.Scan(&usersBookCount); err != nil {
		t.Fatalf("failed to scan row: %v", err)
	}

	if usersBookCount != count {
		t.Errorf("expected same amount of user-books as books, got %d", usersBookCount)
	}
}
