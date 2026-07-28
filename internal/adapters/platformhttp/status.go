package platformhttp

import (
	"encoding/json"
	"fmt"
)

// StatusError переводит неуспешный ответ платформы. Текст из тела полезнее
// кода: платформа уже сформулировала, что именно не так и где чинить — при
// подключении кластера она отвечает «сертификат не сошёлся» или «прав не
// хватает», а не просто «400».
func StatusError(statusCode int, status string, body []byte) error {
	if detail := errorDetail(body); detail != "" {
		return fmt.Errorf("платформа отклонила запрос: %s", detail)
	}
	return fmt.Errorf("платформа ответила %s", status)
}

// errorDetail достаёт человеческую часть из ошибки платформы (RFC 7807).
func errorDetail(body []byte) string {
	var problem struct {
		Detail string `json:"detail"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		return ""
	}
	if problem.Detail != "" {
		return problem.Detail
	}
	return problem.Title
}
