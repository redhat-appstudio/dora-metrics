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
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
	"k8s.io/klog/v2"
)

type GithubClient struct {
	gh    *github.Client
	token string
}

func NewGithubClient() (*GithubClient, error) {
	key := "GITHUB_TOKEN"
	val, ok := os.LookupEnv(key)
	if !ok {
		klog.Errorf("%s not set\n", key)
		return nil, fmt.Errorf("%s not set", key)
	}
	if val == "" {
		klog.Errorf("%s is empty\n", key)
	}

	gh_client := &GithubClient{}

	gh := gh_client.InitClient(val)
	gh_client.gh = gh
	gh_client.token = val

	return gh_client, nil
}

func (gc *GithubClient) InitClient(val string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: val},
	)
	tc := oauth2.NewClient(ctx, ts)

	gh := github.NewClient(tc)
	return gh
}

func (gc *GithubClient) Client() *github.Client {
	return gc.gh
}

func (gc *GithubClient) SearchCommit(hash string, org string) (*github.Commit, error) {
	query := "hash:" + hash + " is:public"
	if len(org) > 0 {
		query = query + " org:" + org
	}
	commits, _, err := gc.Client().Search.Commits(context.Background(), query, &github.SearchOptions{})
	if err != nil {
		//fmt.Println("Search error: ", err)
		return nil, err
	}
	if commits.GetTotal() == 0 {
		return nil, fmt.Errorf("error getting commit: no data found for %s", hash)
	}
	if commits.GetTotal() != 1 {
		return nil, fmt.Errorf("error getting commit: not unique data returned for %s", hash)
	}

	commit := commits.Commits[0].Commit
	return commit, nil
}

func (gc *GithubClient) GetCommitFromOrgAndRepo(org string, repo string, hash string) (*github.Commit, error) {
	searchOrg := org
	new_org := gc.LookupOrg(repo)
	if new_org != "" {
		searchOrg = new_org
	}
	commits, _, err := gc.Client().Repositories.GetCommit(context.Background(), searchOrg, repo, hash, &github.ListOptions{})

	if err != nil {
		//fmt.Println("Can't get ", hash, " use search instead")
		return nil, err
	}

	commit := commits.Commit
	return commit, nil
}

func (gc *GithubClient) LookupOrg(repo string) string {

	repos := map[string]string{
		"pipeline-service-exporter": "openshift-pipelines",
	}
	val, ok := repos[repo]
	if !ok {
		return ""
	}
	return val
}
