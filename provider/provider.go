package provider

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Silo-Server/silo-plugin-ebook-metadata/metadata"
)

const (
	providerTimeout = 10 * time.Second
	searchWorkers   = 4
)

type Source interface {
	ID() string
	Search(context.Context, metadata.SearchQuery) ([]metadata.Match, error)
	Fetch(context.Context, string) (*metadata.Match, error)
}

type Provider struct {
	sources []Source
	byID    map[string]Source
}

func NewProvider() *Provider {
	return NewProviderWithSources(defaultSources())
}

func NewProviderWithSources(sources []Source) *Provider {
	p := &Provider{
		sources: make([]Source, 0, len(sources)),
		byID:    make(map[string]Source, len(sources)),
	}

	for _, source := range sources {
		if source == nil {
			continue
		}
		id := strings.TrimSpace(source.ID())
		if id == "" {
			continue
		}
		p.sources = append(p.sources, source)
		p.byID[id] = source
	}

	return p
}

func (p *Provider) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.Match, error) {
	type result struct {
		source  string
		matches []metadata.Match
		err     error
	}

	results := make(chan result, len(p.sources))
	sem := make(chan struct{}, searchWorkers)

	var wg sync.WaitGroup
	for _, source := range p.sources {
		wg.Add(1)
		go func(source Source) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tctx, cancel := context.WithTimeout(ctx, providerTimeout)
			defer cancel()

			matches, err := source.Search(tctx, q)
			results <- result{
				source:  strings.TrimSpace(source.ID()),
				matches: matches,
				err:     err,
			}
		}(source)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var matches []metadata.Match
	for result := range results {
		if result.err != nil {
			log.Printf("ebook-metadata: provider %s search error: %v", result.source, result.err)
			continue
		}
		matches = append(matches, result.matches...)
	}

	return matches, nil
}

func (p *Provider) Fetch(ctx context.Context, q metadata.SearchQuery) (*metadata.Match, error) {
	tctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	for _, source := range p.sources {
		sourceID := strings.TrimSpace(source.ID())
		providerID := strings.TrimSpace(q.ProviderIDs[sourceID])
		if sourceID == "" || providerID == "" {
			continue
		}
		return source.Fetch(tctx, providerID)
	}

	if sourceID, providerID := metadata.ParseCapabilityProviderID(q.ProviderIDs[metadata.CapabilityID]); sourceID != "" {
		if source := p.byID[sourceID]; source != nil {
			return source.Fetch(tctx, providerID)
		}
	}

	isbn := metadata.NormalizeISBN(q.ProviderIDs["isbn"])
	if isbn == "" {
		return nil, nil
	}
	for _, sourceID := range []string{"openlibrary", "googlebooks", "isbndb"} {
		source := p.byID[sourceID]
		if source == nil {
			continue
		}
		match, err := source.Fetch(tctx, isbn)
		if match != nil || err != nil {
			return match, err
		}
	}

	return nil, nil
}

func defaultSources() []Source {
	return nil
}
