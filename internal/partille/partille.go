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
	"regexp"
	"strconv"
	"time"

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

		`create table if not exists partille_book_status (
			goodreads_book_id int not null unique,
			is_available boolean not null,
			last_fetched_at text not null
		)`,
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

		if _, err := os.Stat(path.Join(configDir, "in-medias-res")); os.IsNotExist(err) {
			err := os.Mkdir(path.Join(configDir, "in-medias-res"), 0700)
			if err != nil {
				return nil, fmt.Errorf("opendb: failed to create config dir: %w", err)
			}
		}

		filename = path.Join(configDir, "in-medias-res", filename)
	}
	filename = filename + "?integrity_check=1&_journal_mode=WAL"
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, fmt.Errorf("opendb: failed to open sqlite memory db: %w, filename: %s", err, filename)
	}
	err = migrate(db)
	if err != nil {
		return nil, fmt.Errorf("opendb: failed to migrate db: %w, filename: %s", err, filename)
	}
	return &Storage{
		db: db,
	}, nil
}

func (s *Storage) Close() {
	s.db.Close()
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

type BookQuery struct {
	Id     int
	Author string
	Title  string
}

type BookAvailabilityPoller func(q BookQuery) (*PollResult, error)
type BookAvailabilityChecker func() ([]BookQuery, error)

// RefreshBookAvailabilities looks for books in the database that needs to be
// checked for availability, and uses the poller function to find out the status
func (s *Storage) RefreshBookAvailabilities(poller BookAvailabilityPoller) error {
	// only update things every 10 days, this Should Be Enough For Everyone
	thresholdDate := time.Now().UTC().Add(-24 * time.Hour * 10).Format(time.RFC3339)

	rows, err := s.db.Query(`select g.rowid, g.title, g.author
	from goodread_books g

	left join partille_book_status p
	on g.rowid = p.goodreads_book_id

	where p.goodreads_book_id is null
	or p.last_fetched_at < ?

	order by g.average_rating desc
	`, thresholdDate)

	if err != nil {
		return fmt.Errorf("refreshbookavailabilities: failed to query db: %w", err)
	}

	defer rows.Close()

	queries := make([]BookQuery, 0)
	for rows.Next() {
		query := BookQuery{}
		err := rows.Scan(query.Id, query.Title, query.Author)
		if err != nil {
			return fmt.Errorf("refreshbookavailabilities: failed to scan row: %w", err)
		}
		queries = append(queries, query)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("refreshbookavailabilities: failed to iterate rows: %w", err)
	}

	if len(queries) == 0 {
		fmt.Println("refreshbookavailabilities: no rows to update, either all are up to date or there are no books in the db")
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, query := range queries {
		pollResult, err := poller(query)
		if err != nil {
			return fmt.Errorf("refreshbookavailabilities: failed to poll: %w", err)
		}
		isAvailable := false
		if pollResult != nil {
			isAvailable = pollResult.IsAvailable
		}
		_, err = s.db.Exec(`
		update partille_book_status
		set last_fetched_at = ?,
		available = ?
		where goodreads_book_id = ?
		`, now, isAvailable, query.Id)
		if err != nil {
			return fmt.Errorf("refreshbookavailabilities: failed to store availability: %w for query: %+v and result: %+v", err, query, pollResult)
		}
	}

	return nil
}

type PollResult struct {
	Title string
	Url   string
	// the consumer must check IsAvailable first (consider it a Maybe/Option)
	IsAvailable bool
}

// PollPartilleBibliotek returns a book from the Partille Bibliotek,
// which you should match against the book you're looking for, probably
// coming from Goodreads
func PollPartilleBibliotek(q BookQuery) (*PollResult, error) {
	pbUrl := "https://bibliotekskatalog.partille.se/cgi-bin/koha/opac-search.pl?advsearch=1&idx=au%2Cwrdl&q=raymond+chandler&op=AND&idx=ti&q=the+big+sleep&weight_search=on&sort_by=popularity_dsc&do=Search"
	searchUrl, err := url.Parse(pbUrl)
	if err != nil {
		// this must never happen, so we panic
		panic(err)
	}
	query := searchUrl.Query()
	query.Set("advsearch", "1")
	query.Add("idx", "au,wrdl")
	query.Add("q", q.Author)
	query.Set("op", "AND")
	query.Add("idx", "ti")
	query.Add("q", q.Title)
	query.Set("weight_search", "on")
	query.Set("sort_by", "popularity_dsc")
	query.Set("sort_by", "popularity_dsc")

	searchUrl.RawQuery = query.Encode()
	fmt.Println("got this as a query", searchUrl.RawQuery)

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
	if book == nil {
		return nil, fmt.Errorf("no book could be parsed from %s", searchUrl.String())
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
