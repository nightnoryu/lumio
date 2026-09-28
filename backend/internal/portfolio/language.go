package portfolio

import "lumio/internal/domain"

func pageLanguage(d domain.Draft) string {
	if d.Language == "ru" {
		return "ru"
	}
	return "en"
}

func localize(d domain.Draft, text string) string {
	if pageLanguage(d) == "ru" {
		if translated, ok := russian[text]; ok {
			return translated
		}
	}
	return text
}

var russian = map[string]string{
	"Skip to photographs":    "Перейти к фотографиям",
	"Contact":                "Контакты",
	"Full-screen photograph": "Фотография на весь экран",
	"Close":                  "Закрыть",
	"Previous photograph":    "Предыдущая фотография",
	"Previous":               "Назад",
	"Next photograph":        "Следующая фотография",
	"Next":                   "Далее",
	"Portfolio photographs":  "Фотографии портфолио",
	"Services and pricing":   "Услуги и цены",
	"Services":               "Услуги",
	"View photograph: ":      "Посмотреть фотографию: ",
	"Photography":            "Фотография",
	"Photography by ":        "Автор фотографии: ",
}
