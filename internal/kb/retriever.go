package kb

import (
	"sort"
	"strings"

	"cortex-cc/internal/models"
	"cortex-cc/internal/store"
)

// Retriever performs BM25-lite keyword retrieval over KB articles stored in SQLite.
// No external services or models are required — retrieval is pure Go.
type Retriever struct {
	store *store.Store
}

func NewRetriever(st *store.Store) *Retriever {
	return &Retriever{store: st}
}

// Search returns the top-K articles most relevant to the query.
func (r *Retriever) Search(query string, topK int) ([]*models.KBArticle, error) {
	articles, err := r.store.ListKBArticles()
	if err != nil || len(articles) == 0 {
		return nil, err
	}

	type scored struct {
		article *models.KBArticle
		score   float64
	}

	qTerms := tokenize(query)
	if len(qTerms) == 0 {
		return nil, nil
	}

	var results []scored
	for _, a := range articles {
		s := bm25(qTerms, a.Title+" "+a.Title+" "+a.Content) // title weighted 2×
		if s > 0 {
			results = append(results, scored{a, s})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if len(results) > topK {
		results = results[:topK]
	}

	out := make([]*models.KBArticle, len(results))
	for i, r := range results {
		out[i] = r.article
	}
	return out, nil
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "can": true, "could": true, "should": true,
	"may": true, "might": true, "how": true, "what": true, "when": true,
	"where": true, "which": true, "who": true, "with": true, "from": true,
}

func tokenize(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, `.,?!;:'"()-`)
		if len(w) > 2 && !stopWords[w] {
			counts[w]++
		}
	}
	return counts
}

// bm25 computes a simplified BM25 score (no IDF — corpus is small).
func bm25(queryTerms map[string]int, docText string) float64 {
	const k1, b, avgLen = 1.5, 0.75, 150.0

	docTerms := tokenize(docText)
	var docLen float64
	for _, cnt := range docTerms {
		docLen += float64(cnt)
	}

	var score float64
	for term := range queryTerms {
		tf := float64(docTerms[term])
		if tf == 0 {
			continue
		}
		norm := tf * (k1 + 1) / (tf + k1*(1-b+b*docLen/avgLen))
		score += norm
	}
	return score
}
