package main

import (
	"strings"
)

// tagList is a custom flag type for comma-separated tags
type tagList []string

func (t *tagList) String() string {
	return strings.Join(*t, ",")
}

func (t *tagList) Set(value string) error {
	*t = []string{} // Clear existing values
	if value == "" {
		return nil
	}
	
	// Split by comma and trim whitespace
	tags := strings.Split(value, ",")
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			*t = append(*t, trimmed)
		}
	}
	return nil
}
