package partille

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

// migrate() applies structural changes and data migrations to the database
func migrate(db *sql.DB) error {
	// if we want to, we could add a "changelog" table or such,
	// to only apply migrations that haven't been applied yet
	stmts := []string{
		`create table if not exists goodread_books (
			title text not null,
			author text not null,
			isbn text not null unique,
			isbn13 text not null unique,
			average_rating text not null,
			original_publication_year int not null
		) strict`,

		`create table if not exists users (
			username text not null check(length(trim(username)) > 0),
			image blob default null
		) strict`,
	}

	for i, stmt := range stmts {
		_, err := db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("migrate: failed to execute statement %d: %w", i, err)
		}
	}

	return nil
}

// NewStorage opens a sqlite database in the user config dir
//
// For test purposes, you can pass the string ":memory:",
// which poorly hides the fact that it's a sqlite db
func NewStorage(filename string) (*Storage, error) {
	if filename != ":memory:" {
		// it's not really a "config" but rather data,
		// not sure where to find a better dir that I dont
		// have to think about
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("opendb: failed to get user config dir: %w", err)
		}
		filename = path.Join(configDir, "partille-goodreads", filename)
	}
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, fmt.Errorf("opendb: failed to open sqlite memory db: %w", err)
	}
	err = migrate(db)
	if err != nil {
		return nil, fmt.Errorf("opendb: failed to migrate db: %w", err)
	}
	return &Storage{
		db: db,
	}, nil
}

func (s *Storage) ImportGoodreadsCsv(reader *csv.Reader) error {
	first := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("csv read error: %w", err)
		}

		if first {
			// skip the header row
			first = false
			continue
		}

		fmt.Println(record)

		// 16: bookshelves
		if record[16] != "to-read" {
			// we're only interested in unread books
			continue
		}

		// interesting columns:
		// 1: title
		// 2: author (first and last name in same column)
		// 5: isbn
		// 6: isbn13
		// 8: average rating
		// 13: original publication year
		// 16: bookshelves
		s.db.Exec(`insert into goodread_books
		(title, author, isbn, isbn13, average_rating, original_publication_year)
		values (?, ?, ?, ?, ?, ?)`, record[1], record[2], record[5], record[6], record[8], record[13])
	}

	return nil
}
