# partille-goodreads

See which of your to-read books are available to borrow, at Partille library.

1. export goodreads list, as csv
    - might want to replace this later with "enter your username", since https://www.goodreads.com/review/list/2793247-carl-helmertz?shelf=to-read is a public URL
1. filter list based on `Bookshelves=to-read` (I think this is generic for Goodreads setup, I have a pretty old account though)
1. order books by rating
    - best metric I can come up with quickly
1. for each ordered goodreads-book:
    1. look in cache
        - if there are zero results, we skip searching again 
    1. remove part of title in `(parenthesis)`, which goodreads usually use for annotating books as part of a serie, i.e. `(Harry Hole, #13)` for the title `Blodmåne`
    1. search through `https://bibliotekskatalog.partille.se/cgi-bin/koha/opac-search.pl?idx=&q=Jo+Nesb%C3%B8+Blodm%C3%A5ne&weight_search=1`
    1. parse the results in the resulting html, including availability and format (e-book, audio, "regular" book)
    1. store the results in the cache, even for empty results

## technical parts

golang and sqlite, might try out htmx/alpinejs for the first time

## wonts

- Use the [goodreads API](https://www.goodreads.com/api), which lacks a lot (response format?) and requires a developer key or oauth etc. Too much hassle