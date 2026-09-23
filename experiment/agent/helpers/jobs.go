/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"sigs.k8s.io/prow/pkg/config"
)

type job struct {
	Name     string `json:"name"`
	Repo     string `json:"repo"`
	Org      string `json:"org"`
	RepoSlug string `json:"repo_slug"`
	JobType  string `json:"job_type"`
}

type output struct {
	Jobs []job `json:"jobs"`
}

func main() {
	configPath := flag.String("config", "config/prow/config.yaml", "Path to the Prow configuration file")
	jobConfigPath := flag.String("job-config", "config/jobs", "Path to the Prow job configuration directory")
	org := flag.String("org", "", "Only include jobs for this organization")
	repo := flag.String("repo", "", "Only include jobs for this repository")
	flag.Parse()

	prowConfig, err := config.LoadStrict(*configPath, *jobConfigPath, nil, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load Prow configuration: %v\n", err)
		os.Exit(1)
	}

	result := output{Jobs: collectJobs(prowConfig, *org, *repo)}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode jobs: %v\n", err)
		os.Exit(1)
	}
}

func collectJobs(prowConfig *config.Config, orgFilter, repoFilter string) []job {
	jobs := []job{}

	for repoPath, presubmits := range prowConfig.PresubmitsStatic {
		org, repo := splitRepoPath(repoPath)
		for _, presubmit := range presubmits {
			jobs = append(jobs, newJob(presubmit.Name, org, repo, "presubmit"))
		}
	}

	for repoPath, postsubmits := range prowConfig.PostsubmitsStatic {
		org, repo := splitRepoPath(repoPath)
		for _, postsubmit := range postsubmits {
			jobs = append(jobs, newJob(postsubmit.Name, org, repo, "postsubmit"))
		}
	}

	for _, periodic := range prowConfig.AllPeriodics() {
		var org, repo string
		if len(periodic.ExtraRefs) > 0 {
			org = periodic.ExtraRefs[0].Org
			repo = periodic.ExtraRefs[0].Repo
		} else {
			org = periodic.Labels["prow.k8s.io/refs.org"]
			repo = periodic.Labels["prow.k8s.io/refs.repo"]
		}
		jobs = append(jobs, newJob(periodic.Name, org, repo, "periodic"))
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Name != jobs[j].Name {
			return jobs[i].Name < jobs[j].Name
		}
		if jobs[i].JobType != jobs[j].JobType {
			return jobs[i].JobType < jobs[j].JobType
		}
		if jobs[i].Org != jobs[j].Org {
			return jobs[i].Org < jobs[j].Org
		}
		return jobs[i].Repo < jobs[j].Repo
	})

	return filterJobs(jobs, orgFilter, repoFilter)
}

func filterJobs(jobs []job, orgFilter, repoFilter string) []job {
	if orgFilter == "" && repoFilter == "" {
		return jobs
	}

	filtered := make([]job, 0, len(jobs))
	for _, job := range jobs {
		if orgFilter != "" && job.Org != orgFilter {
			continue
		}
		if repoFilter != "" && job.Repo != repoFilter {
			continue
		}
		filtered = append(filtered, job)
	}
	return filtered
}

func newJob(name, org, repo, jobType string) job {
	repoSlug := ""
	if org != "" && repo != "" {
		repoSlug = org + "/" + repo
	}
	return job{
		Name:     name,
		Repo:     repo,
		Org:      org,
		RepoSlug: repoSlug,
		JobType:  jobType,
	}
}

func splitRepoPath(repoPath string) (org, repo string) {
	org, repo, _ = strings.Cut(repoPath, "/")
	return org, repo
}
