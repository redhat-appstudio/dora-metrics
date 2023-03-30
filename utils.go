//
// Copyright (c) 2023 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"regexp"
	"strings"
	"time"

	"k8s.io/klog/v2"
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

	klog.V(1).Infof("image %s is filtered out", image)
	return false
}

func excludeImage(excludes []string, image string) bool {
	for _, prefix := range excludes {
		if strings.HasPrefix(image, prefix) {
			klog.V(1).Infof("image %s is excluded", image)
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
