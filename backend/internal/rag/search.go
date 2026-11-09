package rag

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rbac"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

type KnowledgeBase struct {
	mu       sync.RWMutex
	articles []domain.KnowledgeArticle
}

type KnowledgeSeedFile struct {
	Disclaimer string                    `json:"disclaimer"`
	Articles   []domain.KnowledgeArticle `json:"articles"`
}

func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		articles: []domain.KnowledgeArticle{},
	}
}

// LoadFromJSON loads articles from a JSON file (e.g. data/knowledge/avito_playbook_seed.json)
func (kb *KnowledgeBase) LoadFromJSON(path string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var seed KnowledgeSeedFile
	if err := json.Unmarshal(data, &seed); err != nil {
		return err
	}

	kb.articles = seed.Articles
	return nil
}

// SetArticles directly injects articles (used for testing or in-memory seeds)
func (kb *KnowledgeBase) SetArticles(articles []domain.KnowledgeArticle) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.articles = articles
}

// SearchResult contains matches and RBAC statistics
type SearchResult struct {
	Articles           []domain.KnowledgeArticle
	Citations          []domain.KnowledgeCitation
	HiddenByRBACCount  int
	TotalArticlesCount int
}

// Search performs context search filtering by user role
func (kb *KnowledgeBase) Search(role domain.Role, query string, limit int) SearchResult {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))
	keywords := strings.Fields(queryLower)

	type scoredArticle struct {
		article domain.KnowledgeArticle
		score   int
	}

	var scored []scoredArticle
	hiddenCount := 0

	for _, art := range kb.articles {
		// RBAC Filter
		if !rbac.CanAccessKnowledge(role, art.Sensitivity) {
			hiddenCount++
			continue
		}

		if len(keywords) == 0 {
			scored = append(scored, scoredArticle{article: art, score: 1})
			continue
		}

		// Scoring based on title, tags, summary, content
		score := 0
		titleLower := strings.ToLower(art.Title)
		summaryLower := strings.ToLower(art.Summary)
		contentLower := strings.ToLower(art.Content)

		for _, kw := range keywords {
			if strings.Contains(titleLower, kw) {
				score += 5
			}
			for _, tag := range art.Tags {
				if strings.Contains(strings.ToLower(tag), kw) {
					score += 4
				}
			}
			if strings.Contains(summaryLower, kw) {
				score += 3
			}
			if strings.Contains(contentLower, kw) {
				score += 1
			}
		}

		if score > 0 {
			scored = append(scored, scoredArticle{article: art, score: score})
		}
	}

	// Sort scored (simple bubble/insertion sort for small collections)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	var matchedArticles []domain.KnowledgeArticle
	var citations []domain.KnowledgeCitation

	count := len(scored)
	if limit > 0 && count > limit {
		count = limit
	}

	for i := 0; i < count; i++ {
		art := scored[i].article
		matchedArticles = append(matchedArticles, art)
		citations = append(citations, domain.KnowledgeCitation{
			ID:          art.ID,
			Title:       art.Title,
			Source:      art.Source,
			Sensitivity: art.Sensitivity,
		})
	}

	return SearchResult{
		Articles:           matchedArticles,
		Citations:          citations,
		HiddenByRBACCount:  hiddenCount,
		TotalArticlesCount: len(kb.articles),
	}
}

// ListAllVisible returns all articles allowed for a given role
func (kb *KnowledgeBase) ListAllVisible(role domain.Role) ([]domain.KnowledgeArticle, int) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	var visible []domain.KnowledgeArticle
	hiddenCount := 0

	for _, art := range kb.articles {
		if rbac.CanAccessKnowledge(role, art.Sensitivity) {
			visible = append(visible, art)
		} else {
			hiddenCount++
		}
	}

	return visible, hiddenCount
}
