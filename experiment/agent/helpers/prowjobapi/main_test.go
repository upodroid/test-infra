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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testProwJobs = `{"items":[
  {"metadata":{"name":"run-1","labels":{"prow.k8s.io/refs.org":"kubernetes","prow.k8s.io/refs.repo":"test-infra"}},"spec":{"job":"pull-test-infra-unit","type":"presubmit"},"status":{"state":"success"}},
  {"metadata":{"name":"run-2","labels":{"prow.k8s.io/refs.org":"kubernetes","prow.k8s.io/refs.repo":"kubernetes"}},"spec":{"job":"ci-kubernetes-unit","type":"periodic"},"status":{"state":"failure"}}
]}`

func TestLoadProwJobs(t *testing.T) {
	store, err := loadProwJobs(strings.NewReader(testProwJobs))
	if err != nil {
		t.Fatalf("loadProwJobs returned an error: %v", err)
	}
	if len(store.jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(store.jobs))
	}
	if _, found := store.byName["run-1"]; !found {
		t.Error("run-1 is missing from the name index")
	}
}

func TestAPI(t *testing.T) {
	store, err := loadProwJobs(strings.NewReader(testProwJobs))
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name       string
		path       string
		wantStatus int
		wantItems  int
	}{
		{name: "health", path: "/healthz", wantStatus: http.StatusOK},
		{name: "all jobs", path: "/prowjobs", wantStatus: http.StatusOK, wantItems: 2},
		{name: "filtered jobs", path: "/prowjobs?org=kubernetes&repo=test-infra&state=success", wantStatus: http.StatusOK, wantItems: 1},
		{name: "job by name", path: "/prowjobs/run-2", wantStatus: http.StatusOK},
		{name: "missing job", path: "/prowjobs/missing", wantStatus: http.StatusNotFound},
	}

	handler := newHandler(store)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("got status %d, want %d", response.Code, testCase.wantStatus)
			}
			if testCase.wantItems > 0 {
				var got prowJobList
				if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(got.Items) != testCase.wantItems {
					t.Errorf("got %d items, want %d", len(got.Items), testCase.wantItems)
				}
			}
		})
	}
}
