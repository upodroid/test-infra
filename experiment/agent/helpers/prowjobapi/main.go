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
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	defaultProwJobsFile = "experiment/agent/helpers/prowjobs.js"
	maxFileSize         = 512 << 20
)

type prowJobList struct {
	Items []json.RawMessage `json:"items"`
}

type prowJobFields struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Job  string `json:"job"`
		Type string `json:"type"`
	} `json:"spec"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
}

type storedProwJob struct {
	raw    json.RawMessage
	name   string
	job    string
	org    string
	repo   string
	typeID string
	state  string
}

type prowJobStore struct {
	jobs   []storedProwJob
	byName map[string]json.RawMessage
}

func main() {
	listenAddress := flag.String("listen", ":8080", "Address on which to serve the API")
	prowJobsFile := flag.String("prowjobs-file", defaultProwJobsFile, "Path to the prowjobs.js snapshot loaded at startup")
	flag.Parse()

	store, err := loadProwJobsFile(*prowJobsFile)
	if err != nil {
		log.Fatalf("load ProwJobs: %v", err)
	}

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           newHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		log.Printf("serving %d ProwJobs on %s", len(store.jobs), *listenAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve API: %v", err)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("shut down API: %v", err)
	}
}

func loadProwJobsFile(path string) (*prowJobStore, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if fileInfo.Size() > maxFileSize {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, maxFileSize)
	}

	store, err := loadProwJobs(file)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return store, nil
}

func loadProwJobs(reader io.Reader) (*prowJobStore, error) {
	contents, err := io.ReadAll(io.LimitReader(reader, maxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	if len(contents) > maxFileSize {
		return nil, fmt.Errorf("snapshot is larger than %d bytes", maxFileSize)
	}

	var list prowJobList
	if err := json.Unmarshal(contents, &list); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	store := &prowJobStore{
		jobs:   make([]storedProwJob, 0, len(list.Items)),
		byName: make(map[string]json.RawMessage, len(list.Items)),
	}
	for index, raw := range list.Items {
		var fields prowJobFields
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("decode item %d: %w", index, err)
		}
		stored := storedProwJob{
			raw:    raw,
			name:   fields.Metadata.Name,
			job:    fields.Spec.Job,
			org:    fields.Metadata.Labels["prow.k8s.io/refs.org"],
			repo:   fields.Metadata.Labels["prow.k8s.io/refs.repo"],
			typeID: fields.Spec.Type,
			state:  fields.Status.State,
		}
		store.jobs = append(store.jobs, stored)
		store.byName[stored.name] = raw
	}

	return store, nil
}

func newHandler(store *prowJobStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeMethodNotAllowed(response, http.MethodGet)
			return
		}
		writeJSON(response, http.StatusOK, struct {
			Status string `json:"status"`
			Jobs   int    `json:"jobs"`
		}{Status: "ok", Jobs: len(store.jobs)})
	})
	mux.HandleFunc("/prowjobs", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeMethodNotAllowed(response, http.MethodGet)
			return
		}
		writeJSON(response, http.StatusOK, prowJobList{Items: store.filter(request)})
	})
	mux.HandleFunc("/prowjobs/", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeMethodNotAllowed(response, http.MethodGet)
			return
		}
		name := strings.TrimPrefix(request.URL.Path, "/prowjobs/")
		if name == "" || strings.Contains(name, "/") {
			http.NotFound(response, request)
			return
		}
		raw, found := store.byName[name]
		if !found {
			writeJSON(response, http.StatusNotFound, map[string]string{"error": "ProwJob not found"})
			return
		}
		writeRawJSON(response, http.StatusOK, raw)
	})
	return mux
}

func (store *prowJobStore) filter(request *http.Request) []json.RawMessage {
	query := request.URL.Query()
	filters := map[string]string{
		"name":  query.Get("name"),
		"job":   query.Get("job"),
		"org":   query.Get("org"),
		"repo":  query.Get("repo"),
		"type":  query.Get("type"),
		"state": query.Get("state"),
	}

	result := make([]json.RawMessage, 0, len(store.jobs))
	for _, job := range store.jobs {
		if filters["name"] != "" && filters["name"] != job.name ||
			filters["job"] != "" && filters["job"] != job.job ||
			filters["org"] != "" && filters["org"] != job.org ||
			filters["repo"] != "" && filters["repo"] != job.repo ||
			filters["type"] != "" && filters["type"] != job.typeID ||
			filters["state"] != "" && filters["state"] != job.state {
			continue
		}
		result = append(result, job.raw)
	}
	return result
}

func writeMethodNotAllowed(response http.ResponseWriter, allowedMethod string) {
	response.Header().Set("Allow", allowedMethod)
	writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeRawJSON(response http.ResponseWriter, status int, value json.RawMessage) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if _, err := response.Write(append(value, '\n')); err != nil {
		log.Printf("write response: %v", err)
	}
}
