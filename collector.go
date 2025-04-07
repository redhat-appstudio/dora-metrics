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
	"flag"
	"time"

	"github.com/albarbaro/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const CLUSTER_NAME string = "local_demo_cluster"
const APP_LABEL string = "app.kubernetes.io/instance"

// Define a struct for you collector that contains pointers to prometheus descriptors for each metric you wish to expose.
// You can also include fields of other types if they provide utility
type Collector struct {
	commitTimeMetric         *prometheus.Desc
	deployTimeMetric         *prometheus.Desc
	activeDeploymentMetric   *prometheus.Desc
	inactiveDeploymentMetric *prometheus.Desc
	failure_creation_time    *prometheus.Desc
	failure_resolution_time  *prometheus.Desc
	githubClient             *GithubClient
	kubeClient               *KubeClients
	pagerdutyClient          *pagerduty.Client
	commitHashSet            map[string]bool
	gitCache                 map[string]*time.Time
	searchLabel              string
	imageFilter              []string
	imageExcludes            []string
}

// You must create a constructor for you collector that initializes every descriptor and returns a pointer to the collector
func NewCommitTimeCollector() (*Collector, error) {
	// Initialize the github client
	gh, err := NewGithubClient()
	if err != nil {
		return nil, err
	}

	// Initialize the kubernetes clients (clientset and rest)
	kubeClient, err := NewKubeClient()
	if err != nil {
		return nil, err
	}

	pagerdutyClient := NewPagedutyClient()

	searchLabel := "app.kubernetes.io/instance"
	imageFilters := []string{"quay.io/konflux-ci/", "quay.io/redhat-appstudio/", "quay.io/redhat-appstudio-qe/", "quay.io/stolostrn/", "quay.io/abarbaro/"}
	imageExcludes := []string{"quay.io/redhat-appstudio/gitopsdepl", "quay.io/redhat-appstudio/user-workload"}
	flag.Lookup("v").Value.Set("1")

	klog.V(3).Infof("Using label ", searchLabel)
	klog.V(3).Infof("Using image filters: ", imageFilters)

	return &Collector{
		commitTimeMetric: prometheus.NewDesc("dora:committime",
			"Shows timestamp for a specific commit",
			[]string{"app", "commit_hash", "image", "namespace"}, nil,
		),
		deployTimeMetric: prometheus.NewDesc("dora:deploytime",
			"Shows deployment timestamp for a specific commit",
			[]string{"app", "commit_hash", "image", "namespace"}, nil,
		),
		activeDeploymentMetric: prometheus.NewDesc("dora:deployactive",
			"Shows the active deplyment's timestamp in time",
			[]string{"app", "commit_hash", "image", "namespace"}, nil,
		),
		inactiveDeploymentMetric: prometheus.NewDesc("dora:deployinactive",
			"Shows the inactive deplyment's timestamp in time",
			[]string{"app", "commit_hash", "image", "namespace"}, nil,
		),
		failure_resolution_time: prometheus.NewDesc("dora:failure_resolution_time",
			"Shows the failures resolution timestamp in time",
			[]string{"app", "id"}, nil,
		),
		failure_creation_time: prometheus.NewDesc("dora:failure_creation_time",
			"Shows the failures creation timestamp in time",
			[]string{"app", "id"}, nil,
		),
		githubClient:    gh,
		kubeClient:      kubeClient,
		pagerdutyClient: pagerdutyClient,
		commitHashSet:   map[string]bool{},
		gitCache:        map[string]*time.Time{},
		searchLabel:     searchLabel,
		imageFilter:     imageFilters,
		imageExcludes:   imageExcludes,
	}, nil
}

// Each and every collector must implement the Describe function. It essentially writes all descriptors to the prometheus desc channel.
func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.commitTimeMetric
	ch <- collector.deployTimeMetric
}

// Collect implements required collect function for all promehteus collectors
func (collector *Collector) Collect(ch chan<- prometheus.Metric) {

	// List all deployments having argocd label app.kubernetes.io/instance
	// Use these deployments to get images and gather deploytime and commit time

	// this map is used at each scraping to avoid parsing an image twice: duplicate metrics are not allowed by the collector and will throw an error
	collector.commitHashSet = map[string]bool{}

	deploymentList, err := collector.kubeClient.ListDeploymentsByLabels(collector.searchLabel)
	if err != nil {
		klog.Error(err)
		return
	}

	collector.CollectFailures(ch)
	// loop through all deployments found
	for _, depl := range deploymentList.Items {
		// and get all container's images
		for _, cont := range depl.Spec.Template.Spec.Containers {
			// filter the images to only use the appstudio ones
			isOk := filterImage(collector.imageFilter, cont.Image)
			isExcluded := excludeImage(collector.imageExcludes, cont.Image)
			if isOk && !isExcluded {
				collector.CollectCommitTime(ch, &depl, &cont)
				collector.CollectDeployTime(ch, &depl, &cont)
				collector.commitHashSet[cont.Image] = true
			}
		}
	}

}

func (collector *Collector) CollectCommitTime(ch chan<- prometheus.Metric, depl *appsv1.Deployment, cont *v1.Container) {
	// check we have not parsed this image already
	// get data needed for prometheus labels
	namespace := depl.Namespace
	component := depl.Labels[collector.searchLabel]
	// parse the image url to extract organization, repository and commit hash
	fields := reSubMatchMap(imageRegex, cont.Image)

	_, ok := collector.commitHashSet[cont.Image]
	if !ok {
		// check if we have already searched for this commit, before requesting github apis
		// if yes, use that value and return
		commitTimeValue, commitCached := collector.gitCache[fields["hash"]]
		if !commitCached {
			klog.V(3).Infof("Commit time is not cached yet: %s %s", fields["repo"], fields["hash"])
		} else {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commitTimeValue.Unix()), component, fields["hash"], cont.Image, namespace)
			// We let prometheus set the scraping timestamp; if we force-set it to the commit time we risk losing old out-of-bound data
			ch <- m1
			klog.V(3).Infof("collected (from cache) committime for %s", cont.Image)
			return
		}

		// if the data is not cached, look in github: first try is using org+repo+hash to directly get the data from the repo (we want to avoid searching for a generic hash)
		commit, err := collector.githubClient.GetCommitFromOrgAndRepo(fields["org"], fields["repo"], fields["hash"])
		if err != nil {
			klog.V(3).Infof("Can't find commit time using %s, %s and %s: %s", fields["org"], fields["repo"], fields["hash"], err)
		} else {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.Unix()), component, fields["hash"], cont.Image, namespace)
			// We let prometheus set the scraping timestamp; if we force-set it to the commit time we risk losing old out-of-bound data
			ch <- m1
			klog.V(3).Infof("collected committime for %s", cont.Image)
			collector.gitCache[fields["hash"]] = commit.Author.Date
			return
		}

		commit, err = collector.githubClient.SearchCommit(fields["hash"], fields["org"])
		if err != nil {
			// try again once
			commit, err = collector.githubClient.SearchCommit(fields["hash"], fields["org"])
			klog.V(3).Infof("Retrying search: %s - %s - %s: %s", fields["repo"], fields["hash"], fields["org"], err)
		}

		if err != nil {
			// try again once
			commit, err = collector.githubClient.SearchCommit(fields["hash"], fields["org"])
			klog.V(1).Infof("Can't find commit either by get or search: %s - %s - %s: %s", fields["repo"], fields["hash"], fields["org"], err)
		} else {
			m1 := prometheus.MustNewConstMetric(collector.commitTimeMetric, prometheus.GaugeValue, float64(commit.Author.Date.Unix()), component, fields["hash"], cont.Image, namespace)
			// We let prometheus set the scraping timestamp; if we force-set it to the commit time we risk losing old out-of-bound data
			ch <- m1
			klog.V(3).Infof("collected committime for %s", cont.Image, ": ", err)
			collector.gitCache[fields["hash"]] = commit.Author.Date
			return
		}

	}

}

func (collector *Collector) CollectDeployTime(ch chan<- prometheus.Metric, depl *appsv1.Deployment, cont *v1.Container) {
	// check we have not parsed this image already
	_, ok := collector.commitHashSet[cont.Image]
	if !ok {
		// get data needed for prometheus labels
		namespace := depl.Namespace
		component := depl.Labels[collector.searchLabel]
		// parse the image url to extract organization, repository and commit hash
		fields := reSubMatchMap(imageRegex, cont.Image)
		// If the deployment is active we also collect the deploy time metric using the deployment creation timestamp
		isActive, _ := collector.kubeClient.IsDeploymentActiveSince(depl)

		if isActive {
			creationTime, err := collector.kubeClient.GetDeploymentReplicaSetCreationTime(namespace, depl.Name, cont.Image)
			if err != nil {
				klog.Error(err)
			} else {

				isOkToIngest := creationTime.After(time.Now().Add(-1 * time.Hour))
				if isOkToIngest {
					m1 := prometheus.MustNewConstMetric(collector.deployTimeMetric, prometheus.GaugeValue, float64(creationTime.Unix()), component, fields["hash"], cont.Image, namespace)
					// We care only deployments collected after install time, so we can force-set the timestamp to the deplytime without loosing any data
					// This will simplify building the metrics in Grafana.
					// Active deplyments with current timestamp are anyways collected by the activeDeploymentMetric right after
					m1 = prometheus.NewMetricWithTimestamp(creationTime.Time, m1)
					ch <- m1
					klog.V(3).Infof("collected deploytime for %s", cont.Image)
				}

				m2 := prometheus.MustNewConstMetric(collector.activeDeploymentMetric, prometheus.GaugeValue, float64(creationTime.Unix()), component, fields["hash"], cont.Image, namespace)
				ch <- m2
				klog.V(3).Infof("collected active deployment for %s", cont.Image)
			}

		} else {
			m2 := prometheus.MustNewConstMetric(collector.inactiveDeploymentMetric, prometheus.GaugeValue, float64(time.Now().Unix()), component, fields["hash"], cont.Image, namespace)
			ch <- m2
			klog.V(3).Infof("collected Inactive deployment for %s", cont.Image)
		}
	}
}

func (collector *Collector) CollectFailures(ch chan<- prometheus.Metric) {
	klog.V(1).Info("Collecting failures...")
	incidents, err := collector.pagerdutyClient.ListIncidentsWithContext(context.TODO(), pagerduty.ListIncidentsOptions{ServiceIDs: []string{"PL93A8P"}})

	if err != nil {
		klog.Error(err)
		return
	}
	for _, inc := range incidents.Incidents {
		layout := "2006-01-02T15:04:05Z"

		creationTime, err := time.Parse(layout, inc.CreatedAt)
		if err != nil {
			klog.Error("error converting time for %s", inc.ID)
		}
		isOkToIngest := creationTime.After(time.Now().Add(-5 * time.Minute))

		if isOkToIngest {
			m2 := prometheus.MustNewConstMetric(collector.failure_creation_time, prometheus.GaugeValue, float64(creationTime.Unix()), inc.ID, "global")
			m2 = prometheus.NewMetricWithTimestamp(creationTime, m2)
			ch <- m2
			klog.V(3).Infof("collected failure creation time for %s", inc.ID)
		}

		if inc.Status == "resolved" {
			resTime, err := time.Parse(layout, inc.ResolvedAt)
			if err != nil {
				klog.Error("error converting time for %s", inc.ID, err)
			}
			isOkToIngest := resTime.After(time.Now().Add(-5 * time.Minute))

			if isOkToIngest {
				m2 := prometheus.MustNewConstMetric(collector.failure_resolution_time, prometheus.GaugeValue, float64(resTime.Unix()), inc.ID, "global")
				m2 = prometheus.NewMetricWithTimestamp(resTime, m2)
				ch <- m2
				klog.V(3).Infof("collected failure resolution time for %s", inc.ID)
			}
		}

	}
}
