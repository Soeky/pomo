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

	if strings.Contains(raw, Delimiter) {
		parts := strings.SplitN(raw, Delimiter, 2)
		return ParseParts(parts[0], parts[1])
	}

	return ParseParts(raw, DefaultSubtopic)
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
	return p.Domain + Delimiter + p.Subtopic
}
