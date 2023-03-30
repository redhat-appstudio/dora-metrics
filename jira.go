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
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	jira "github.com/andygrunwald/go-jira"
)

type Jira interface {
	GetIssueByJQLQuery(JQLQuery string) []jira.Issue
	ParseCreationTime(issue jira.Issue) (*time.Time, error)
	ParseResolutionTime(issue jira.Issue) (*time.Time, error)
}

func NewJiraConfig() (Jira, error) {
	key := "JIRA_TOKEN"
	val, ok := os.LookupEnv(key)
	if !ok {
		fmt.Printf("%s not set\n", key)
		return &clientFactory{}, fmt.Errorf("no JIRA_TOKEN found")
	}
	token := val

	transport := TokenAuthTransport{Token: token}
	client, _ := jira.NewClient(transport.Client(), "https://issues.redhat.com")

	return &clientFactory{
		Client: client,
	}, nil
}

type clientFactory struct {
	Client *jira.Client
}

type TokenAuthTransport struct {
	Token string

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

func (t *TokenAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := cloneRequest(req) // per RoundTripper contract
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.Token))
	return t.transport().RoundTrip(req2)
}

func (t *TokenAuthTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *TokenAuthTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

func (t *clientFactory) GetIssueByJQLQuery(JQLQuery string) []jira.Issue {
	var issues []jira.Issue

	// append the jira issues to []jira.Issue
	appendFunc := func(i jira.Issue) (err error) {
		issues = append(issues, i)
		return err
	}

	// In this example, we'll search for all the issues with the provided JQL filter and Print the Story Points
	err := t.Client.Issue.SearchPages(JQLQuery, nil, appendFunc)
	if err != nil {
		log.Fatal(err)
	}
	return issues
}

func (t *clientFactory) ParseCreationTime(issue jira.Issue) (*time.Time, error) {
	c, err := issue.Fields.Created.MarshalJSON()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	ct, err := time.Parse("\"2006-01-02T15:04:05.999-0700\"", string(c))
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return &ct, nil
}

func (t *clientFactory) ParseResolutionTime(issue jira.Issue) (*time.Time, error) {
	if issue.Fields.Status.Name != "Closed" {
		return nil, fmt.Errorf("request is not in closed state: %s", issue.Fields.Status.Name)
	}
	c, err := issue.Fields.Resolutiondate.MarshalJSON()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	ct, err := time.Parse("\"2006-01-02T15:04:05.999-0700\"", string(c))
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return &ct, nil
}
