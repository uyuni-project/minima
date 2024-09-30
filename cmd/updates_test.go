/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTransport struct {
	responses map[string]*http.Response
}

func (mrt *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if res, ok := mrt.responses[req.URL.String()]; ok {
		return res, nil
	}

	return &http.Response{
		Status:     "404 Not Found",
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}, nil
}

func createMockClient(baseUrl string, archs []string) *http.Client {
	responses := make(map[string]*http.Response, len(archs))

	for _, arch := range archs {
		url := fmt.Sprintf("%s%s/", baseUrl, arch)
		responses[url] = &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}
	}

	return &http.Client{
		Transport: &mockTransport{responses: responses},
	}
}

func TestArchMage(t *testing.T) {
	baseUrl := "http://download.suse.de/ibs/totallyrandomtestrepo/"

	tests := []struct {
		name       string
		url        string
		validArchs []string
		wantErr    bool
	}{
		{"Arch in repo URL", baseUrl + "x86_64/", []string{"x86_64"}, false},
		{"Single available arch for a repo", baseUrl, []string{"aarch64"}, false},
		{"Multiple available archs for a repo", baseUrl, []string{"x86_64", "aarch64", "ppc64le", "s390x"}, false},
		{"No available archs for a repo", baseUrl, []string{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := HTTPRepoConfig{
				URL:   tt.url,
				Archs: []string{},
			}
			mockClient := createMockClient(repo.URL, tt.validArchs)

			err := ArchMage(mockClient, &repo)
			assert.EqualValues(t, tt.wantErr, (err != nil))

			for arch := range tt.validArchs {
				var found bool
				for repoArch := range repo.Archs {
					if arch == repoArch {
						found = true
						break
					}
				}
				assert.True(t, found)
			}
		})
	}
}
