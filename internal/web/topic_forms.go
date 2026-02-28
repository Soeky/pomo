package web

import (
	"net/http"
	"strings"

	"github.com/Soeky/pomo/internal/topics"
)

type parsedTopicForm struct {
	Path     topics.Path
	Provided bool
	Source   string
}

func parseTopicForm(r *http.Request, combinedKeys ...string) (parsedTopicForm, error) {
	domain := strings.TrimSpace(r.FormValue("domain"))
	subtopic := strings.TrimSpace(r.FormValue("subtopic"))
	if domain != "" || subtopic != "" {
		path, err := topics.ParseParts(domain, subtopic)
		if err != nil {
			return parsedTopicForm{}, err
		}
		return parsedTopicForm{
			Path:     path,
			Provided: true,
			Source:   "domain_subtopic",
		}, nil
	}

	for _, key := range combinedKeys {
		raw := strings.TrimSpace(r.FormValue(key))
		if raw == "" {
			continue
		}
		path, err := topics.Parse(raw)
		if err != nil {
			return parsedTopicForm{}, err
		}
		return parsedTopicForm{
			Path:     path,
			Provided: true,
			Source:   key,
		}, nil
	}
	return parsedTopicForm{}, nil
}

func normalizePlannedTitle(rawTitle string, parsed parsedTopicForm) string {
	rawTitle = strings.TrimSpace(rawTitle)
	switch parsed.Source {
	case "domain_subtopic", "topic":
		return parsed.Path.Canonical()
	case "title":
		if strings.Contains(rawTitle, topics.Delimiter) || strings.Contains(rawTitle, `\`+topics.Delimiter) {
			return parsed.Path.Canonical()
		}
		return rawTitle
	default:
		return rawTitle
	}
}
