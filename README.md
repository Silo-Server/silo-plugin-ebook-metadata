# Silo Ebook Metadata Plugin

Standalone metadata provider plugin for Silo ebook libraries.

## Capability

- `metadata_provider.v1`
- Capability ID: `ebook-metadata`
- Default priority: `ebook = 2`

## Sources

OpenLibrary, Google Books, ISBNdb, Hardcover, Goodreads, Amazon, Anna's Archive, Project Gutenberg, BookBrainz, FantasticFiction, ISFDB, LibraryThing, Internet Archive, WorldCat, and Douban.

Google Books, ISBNdb, and Hardcover require API keys before they make upstream requests. The other sources use public APIs or scraped catalog pages.

## Identity

Ebooks use ISBN and source-specific provider IDs. The plugin maps people as authors only.

## Configuration

- `enabled_sources`: comma-separated source IDs. Empty uses the default source set.
- `google_books_api_key`: optional Google Books API key.
- `isbndb_api_key`: optional ISBNdb API key.
- `hardcover_api_key`: optional Hardcover API key.
- `default_region`: optional region hint used by regional sources.

## Development

Run tests:

```sh
go test ./...
```

Build the plugin binary:

```sh
make build
```
