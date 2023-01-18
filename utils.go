package main

import (
	"regexp"
	"strings"
	"time"
)

// Helper function and regex to extract values from an image URL
var imageRegex = regexp.MustCompile(`quay.io\/(?P<org>[-a-zA-Z0-9]*)\/(?P<repo>[-a-zA-Z0-9]*)(@sha256)?:(?P<hash>[-a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]*)`)

func reSubMatchMap(r *regexp.Regexp, str string) map[string]string {
	match := r.FindStringSubmatch(str)
	subMatchMap := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}
	return subMatchMap
}

func filterImage(prefixes []string, image string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(image, prefix) {
			return true
		}
	}
	return false
}

func isDateTooOldForPrometheus(t *time.Time) bool {
	if t == nil {
		return false
	}
	checkDate := time.Now().Add(15 * 24 * time.Hour)
	isDateTooOldForPormetheus := t.Before(checkDate)
	return isDateTooOldForPormetheus
}
