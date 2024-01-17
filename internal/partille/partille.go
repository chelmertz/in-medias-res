package partille

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html"
)

type Storage struct {
	db *sql.DB
}

// migrate() applies structural changes and data migrations to the database
func migrate(db *sql.DB) error {
	// if we want to, we could add a "changelog" table or such (and to hold a
	// lock over the process), to only apply migrations that haven't been
	// applied yet
	stmts := []string{
		`create table if not exists goodread_books (
			title text not null,
			author text not null,
			isbn text default null,
			isbn13 text default null,
			average_rating text not null,
			original_publication_year int not null
		) strict`,

		`create table if not exists users (
			username text not null check(length(trim(username)) > 0) unique,
			image blob default null,
			last_fetched_goodreads_books_at text default null,
			goodreads_id int default null
		) strict`,

		`create table if not exists user_goodread_books (
			user_id int not null,
			book_id int not null
		)`,

		`create unique index if not exists ugb on user_goodread_books(user_id, book_id)`,
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
	filename = filename + "?integrity_check=1&_journal_mode=WAL"
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

func (s *Storage) ImportGoodreadsCsv(reader *csv.Reader, userId int) error {
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
		book := Book{
			Title:         record[1],
			Author:        record[2],
			Isbn:          record[5],
			Isbn13:        record[6],
			AverageRating: record[8],
		}

		originalPublicationYear, err := strconv.Atoi(record[13])
		if err == nil {
			book.OriginalPublicationYear = originalPublicationYear
		}

		bookId, err := s.StoreBook(book)
		if err != nil {
			return fmt.Errorf("failed to store book: %w", err)
		}
		err = s.StoreBookForUser(bookId, userId)
		if err != nil {
			return fmt.Errorf("failed to store book: %w", err)
		}
	}

	return nil
}

type Book struct {
	Title                   string
	Author                  string
	Isbn                    string
	Isbn13                  string
	AverageRating           string
	OriginalPublicationYear int
}

// StoreBook doesn't fail if the book already exists
func (s *Storage) StoreBook(book Book) (int, error) {
	// TODO if duplicate, return original rowid
	res, err := s.db.Exec(`insert into goodread_books
		(title, author, isbn, isbn13, average_rating, original_publication_year)
		values (?, ?, ?, ?, ?, ?)
		returning rowid`, book.Title, book.Author, book.Isbn, book.Isbn13, book.AverageRating, book.OriginalPublicationYear)
	if err != nil {
		return 0, fmt.Errorf("storebook: failed to insert book: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("storebook: failed to get last insert id: %w", err)
	}
	return int(id), nil
}

func (s *Storage) StoreBookForUser(bookId, userId int) error {
	_, err := s.db.Exec(`insert into user_goodread_books
		(user_id, book_id)
		values (?, ?)`, userId, bookId)
	if err != nil {
		return fmt.Errorf("storebookforuser: failed to insert: %w", err)
	}
	return nil
}

type User struct {
	Username string
	Id       int
}

func (s *Storage) CreateUser(username string) (int, error) {
	res, err := s.db.Exec("insert into users (username) values (?) returning rowid", username)
	if err != nil {
		return 0, fmt.Errorf("createuser: failed to insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("createuser: failed to get last insert id: %w", err)
	}
	return int(id), nil
}

type PartilleBibliotekBook struct {
	Title     string
	DetailUrl string
}

// PollPartilleBibliotek returns a book from the Partille Bibliotek,
// which you should match against the book you're looking for, probably
// coming from Goodreads
func PollPartilleBibliotek(searchTerm string) (*PartilleBibliotekBook, error) {
	pbUrl := "https://bibliotekskatalog.partille.se/cgi-bin/koha/opac-search.pl?idx=&sort_by=popularity_dsc&weight_search=1"
	searchUrl, err := url.Parse(pbUrl)
	if err != nil {
		// this must never happen, so we panic
		panic(err)
	}
	query := searchUrl.Query()
	query.Set("q", searchTerm)

	searchUrl.RawQuery = query.Encode()

	// let's assume our result is on the first page, or not there at all
	resp, err := http.Get(searchUrl.String())
	if err != nil {
		return nil, fmt.Errorf("pollpartillebibliotek: failed to get search page: %w", err)
	}
	defer resp.Body.Close()

	node, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pollpartillebibliotek: failed to parse html: %w", err)
	}
	book := firstResult(node)

	return book, nil
}

func firstResult(node *html.Node) *PartilleBibliotekBook {
	book := &PartilleBibliotekBook{}
	foundOne := false

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && attr.Val == "title" {
					foundOne = true
				}
			}
		}

		// continue traversing
		for c := n.FirstChild; c != nil && !foundOne; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(node)

	return &PartilleBibliotekBook{
		Title: "The Hobbit",
	}
}
