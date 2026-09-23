package store

type Category struct {
	Code  string
	Label string
}

var Categories = []Category{
	{"crm", "CRM и продажи"},
	{"automation", "Автоматизация процессов и администрирование"},
	{"analytics", "Аналитика и отчётность"},
	{"ai_assistant", "Чат-бот / AI-ассистент"},
	{"web_app", "Веб-сервис или приложение"},
	{"integration", "Интеграция систем и данных"},
	{"content", "Обучение и контент"},
	{"other", "Другое"},
}

func CategoryLabel(code string) string {
	for _, c := range Categories {
		if c.Code == code {
			return c.Label
		}
	}
	return ""
}

func ValidCategory(code string) bool {
	return code == "" || CategoryLabel(code) != ""
}
