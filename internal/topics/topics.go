package topics

import (
	"fmt"
	"strings"
)

const (
	DefaultDomain   = "General"
	DefaultSubtopic = "General"
	Delimiter       = "::"
)

type Path struct {
	Domain   string
	Subtopic string
}

func Parse(input string) (Path, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Path{Domain: DefaultDomain, Subtopic: DefaultSubtopic}, nil
	}

	if domainRaw, subtopicRaw, hasDelimiter := splitTopicInput(raw); hasDelimiter {
		return ParseParts(unescapeTopicPart(domainRaw), unescapeTopicPart(subtopicRaw))
	}

	return ParseParts(unescapeTopicPart(raw), DefaultSubtopic)
}

func ParseParts(domain, subtopic string) (Path, error) {
	d := strings.TrimSpace(domain)
	s := strings.TrimSpace(subtopic)

	if d == "" {
		return Path{}, fmt.Errorf("topic domain must not be empty")
	}
	if s == "" {
		s = DefaultSubtopic
	}

	return Path{Domain: d, Subtopic: s}, nil
}

func (p Path) Canonical() string {
	return escapeTopicPart(p.Domain) + Delimiter + escapeTopicPart(p.Subtopic)
}

func splitTopicInput(raw string) (domain string, subtopic string, hasDelimiter bool) {
	escaped := false
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			if escaped {
				escaped = false
				continue
			}
			escaped = true
		case ':':
			if escaped {
				escaped = false
				continue
			}
			if i+1 < len(raw) && raw[i+1] == ':' {
				return raw[:i], raw[i+2:], true
			}
		default:
			escaped = false
		}
	}
	return raw, "", false
}

func unescapeTopicPart(input string) string {
	if input == "" {
		return input
	}
	var b strings.Builder
	b.Grow(len(input))

	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(ch)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func escapeTopicPart(input string) string {
	if input == "" {
		return input
	}
	out := strings.ReplaceAll(input, `\`, `\\`)
	out = strings.ReplaceAll(out, Delimiter, `\`+Delimiter)
	return out
}
