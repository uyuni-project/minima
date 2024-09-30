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
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTransport struct {
	responses  map[string]*http.Response
	forceError bool
}

func (mrt *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if mrt.forceError {
		return nil, errors.New("Some kindof network error")
	}

	if res, ok := mrt.responses[req.URL.String()]; ok {
		return res, nil
	}

	return &http.Response{
		Status:     "404 Not Found",
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}, nil
}

func createMockClient(baseUrl string, archs []string, forceError bool) *http.Client {
	responses := make(map[string]*http.Response, len(archs))
	responses[baseUrl] = &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("\"SUSE_SLE-15-SP4_Update/\"\"SLE-15SP5_Update/\"\"SUSE_SLE-15SP6_Update/\"")),
	}

	for _, arch := range archs {
		url := fmt.Sprintf("%s%s/", baseUrl, arch)
		responses[url] = &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}
	}

	return &http.Client{
		Transport: &mockTransport{
			responses:  responses,
			forceError: forceError,
		},
	}
}

func TestArchMage(t *testing.T) {
	baseUrl := "http://download.suse.de/ibs/SUSA:/Maintenance:/11111/SUSE_SLE-15-SP4_Update/"

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
			mockClient := createMockClient(repo.URL, tt.validArchs, false)

			err := ArchMage(mockClient, &repo)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.ElementsMatch(t, tt.validArchs, repo.Archs)
		})
	}
}

func TestProcWebChunk(t *testing.T) {
	tests := []struct {
		name       string
		maint      string
		product    string
		validArchs []string
		want       []HTTPRepoConfig
		wantErr    bool
	}{
		{
			"Valid maint repo - arch in the url", "http://download.suse.de/ibs/SUSE:/Maintenance:/22222/", "SUSE_SLE-15-SP4_Update/x86_64/",
			[]string{},
			[]HTTPRepoConfig{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/22222/SUSE_SLE-15-SP4_Update/x86_64/",
					Archs: []string{"x86_64"},
				},
			},
			false,
		},
		{
			"Valid maint repo - single valid arch", "http://download.suse.de/ibs/SUSE:/Maintenance:/33333/", "SUSE_SLE-15-SP4_Update/",
			[]string{"aarch64"},
			[]HTTPRepoConfig{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/33333/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"aarch64"},
				},
			},
			false,
		},
		{
			"Valid maint repo - multiple valid archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/44444/", "SUSE_SLE-15-SP4_Update/",
			[]string{"x86_64", "aarch64", "ppc64le", "s390x"},
			[]HTTPRepoConfig{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/44444/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"x86_64", "aarch64", "ppc64le", "s390x"},
				},
			},
			false,
		},
		{
			"Valid maint repo - no valid archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/55555/", "SUSE_SLE-15-SP4_Update/",
			[]string{},
			[]HTTPRepoConfig{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s", tt.maint, tt.product)
			client := createMockClient(url, tt.validArchs, false)

			got, err := ProcWebChunk(client, tt.product, tt.maint)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.Equal(t, len(tt.want), len(got))
			for i := range tt.want {
				wantRepo := tt.want[i]
				gotRepo := got[i]
				assert.Equal(t, wantRepo.URL, gotRepo.URL)
				assert.ElementsMatch(t, wantRepo.Archs, gotRepo.Archs)
			}
		})
	}
}

func TestGetProductsForMU(t *testing.T) {
	tests := []struct {
		name    string
		mu      string
		want    []string
		wantErr bool
	}{
		{
			"Chunk without 'SUSE' is discarded",
			"http://download.suse.de/ibs/SUSE:/Maintenance:/11111/",
			[]string{"SUSE_SLE-15-SP4_Update/", "SUSE_SLE-15SP6_Update/"},
			false,
		},
		{
			"Network error",
			"http://download.suse.de/ibs/SUSE:/Maintenance:/11111/",
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockClient(tt.mu, []string{}, tt.wantErr)

			got, err := getProductsForMU(client, tt.mu)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
