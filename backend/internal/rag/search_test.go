package rag_test

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rag"
	"testing"
)

func sampleArticles() []domain.KnowledgeArticle {
	return []domain.KnowledgeArticle{
		{
			ID:          "kb-01",
			Title:       "Получение доступов в IDM",
			Category:    "onboarding",
			Sensitivity: "intern",
			Source:      "Playbook",
			Summary:     "Инструкция по доступам",
			Content:     "Для работы нужен repo:avito-delivery-core",
			Tags:        []string{"доступы", "idm", "репозиторий"},
		},
		{
			ID:          "kb-02",
			Title:       "Оформление отгула",
			Category:    "hr_processes",
			Sensitivity: "intern",
			Source:      "Handbook",
			Summary:     "Как взять учебный отгул",
			Content:     "Подайте заявку в сервис Отсутствий",
			Tags:        []string{"отгул", "absence", "экзамен"},
		},
		{
			ID:          "kb-03",
			Title:       "Чек-лист оценки стажёра на Экваторе",
			Category:    "management",
			Sensitivity: "mentor_only",
			Source:      "Buddy Guide",
			Summary:     "Инструкция для наставника",
			Content:     "Оценка компетенций",
			Tags:        []string{"оценка", "экватор", "наставник"},
		},
		{
			ID:          "kb-04",
			Title:       "Бюджетные вилки грейдов",
			Category:    "management",
			Sensitivity: "manager_only",
			Source:      "Management Confidential",
			Summary:     "Зарплатные сетки",
			Content:     "Конфиденциально",
			Tags:        []string{"зарплата", "бюджет"},
		},
	}
}

func TestKnowledgeBaseSearch(t *testing.T) {
	kb := rag.NewKnowledgeBase()
	kb.SetArticles(sampleArticles())

	// 1. Intern search: should find "доступы" and hide mentor/manager articles
	resIntern := kb.Search(domain.RoleIntern, "доступы", 5)
	if len(resIntern.Articles) == 0 {
		t.Fatalf("expected to find access article for intern")
	}
	if resIntern.Articles[0].ID != "kb-01" {
		t.Errorf("expected kb-01 first, got %s", resIntern.Articles[0].ID)
	}
	if resIntern.HiddenByRBACCount != 2 {
		t.Errorf("expected 2 hidden articles for intern, got %d", resIntern.HiddenByRBACCount)
	}

	// 2. Intern search for forbidden topic "зарплата": should return 0 articles, 2 hidden
	resSalary := kb.Search(domain.RoleIntern, "зарплата", 5)
	if len(resSalary.Articles) != 0 {
		t.Errorf("intern must not see salary article, got %d", len(resSalary.Articles))
	}

	// 3. Manager search for "зарплата": should find article
	resManager := kb.Search(domain.RoleManager, "зарплата", 5)
	if len(resManager.Articles) == 0 {
		t.Fatalf("manager should see salary article")
	}
	if resManager.HiddenByRBACCount != 0 {
		t.Errorf("manager should have 0 hidden articles, got %d", resManager.HiddenByRBACCount)
	}

	// 4. Empty query lists all visible
	resEmpty := kb.Search(domain.RoleIntern, "", 10)
	if len(resEmpty.Articles) != 2 {
		t.Errorf("expected 2 visible articles for empty search by intern, got %d", len(resEmpty.Articles))
	}
}

func TestListAllVisible(t *testing.T) {
	kb := rag.NewKnowledgeBase()
	kb.SetArticles(sampleArticles())

	internVisible, hiddenIntern := kb.ListAllVisible(domain.RoleIntern)
	if len(internVisible) != 2 || hiddenIntern != 2 {
		t.Errorf("intern visible: got %d, hidden: got %d", len(internVisible), hiddenIntern)
	}

	mentorVisible, hiddenMentor := kb.ListAllVisible(domain.RoleMentor)
	if len(mentorVisible) != 3 || hiddenMentor != 1 {
		t.Errorf("mentor visible: got %d, hidden: got %d", len(mentorVisible), hiddenMentor)
	}

	managerVisible, hiddenManager := kb.ListAllVisible(domain.RoleManager)
	if len(managerVisible) != 4 || hiddenManager != 0 {
		t.Errorf("manager visible: got %d, hidden: got %d", len(managerVisible), hiddenManager)
	}
}

func TestLoadFromJSON(t *testing.T) {
	kb := rag.NewKnowledgeBase()
	err := kb.LoadFromJSON("../../../data/knowledge/avito_playbook_seed.json")
	if err != nil {
		t.Fatalf("failed to load seed json: %v", err)
	}

	res := kb.Search(domain.RoleIntern, "отгул", 2)
	if len(res.Articles) == 0 {
		t.Fatalf("expected to find article about отгул from seed json")
	}
}
