/*
Copyright © 2021-2024 NAME HERE <EMAIL ADDRESS>

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
	"io"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uyuni-project/minima/get"
)

var sle15sp4Entry = "SUSE_SLE-15-SP4_Update/"
var sle15sp6Entry = "SUSE_SLE-15-SP6_Update/"
var productEntries = [...]string{sle15sp4Entry, sle15sp6Entry}

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
		Body: io.NopCloser(bytes.NewBufferString(`<a href=\"/ibs/SUSE:/Maintenance:/number/SUSE_SLE-15-SP4_Update/\">SUSE_SLE-15-SP4_Update/</a>
		<a href=\"/ibs/SUSE:/Maintenance:/number/SLE-15-SP4_Update/\">SLE-15-SP5_Update/</a>
		<a href=\"/ibs/SUSE:/Maintenance:/number/SUSE_SLE-15-SP4_Update/\">SUSE_SLE-15-SP6_Update/</a>`)),
	}

	for _, p := range productEntries {
		url := baseUrl + p
		responses[url] = &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}

		for _, arch := range archs {
			archUrl := url + arch + "/"
			responses[archUrl] = &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString("")),
			}
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
	maint := "http://download.suse.de/ibs/SUSE:/Maintenance:/1/"

	tests := []struct {
		name       string
		validArchs []string
		urlArch    string
		wantErr    bool
	}{
		{"Arch in repo URL", []string{"x86_64"}, "x86_64", false},
		{"Single available arch for a repo", []string{"aarch64"}, "", false},
		{"Multiple available archs for a repo", []string{"x86_64", "aarch64", "ppc64le", "s390x"}, "", false},
		{"No available archs for a repo", []string{}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := get.HTTPRepo{
				URL:   maint + sle15sp4Entry + tt.urlArch,
				Archs: []string{},
			}
			mockClient := createMockClient(maint, tt.validArchs, false)

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
		want       []get.HTTPRepo
		netWorkErr bool
		wantErr    bool
	}{
		{
			"Valid maint repo - single valid arch", "http://download.suse.de/ibs/SUSE:/Maintenance:/2/", "SUSE_SLE-15-SP4_Update/",
			[]string{"aarch64"},
			[]get.HTTPRepo{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/2/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"aarch64"},
				},
			},
			false,
			false,
		},
		{
			"Valid maint repo - multiple valid archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/3/", "SUSE_SLE-15-SP4_Update/",
			[]string{"x86_64", "aarch64", "ppc64le", "s390x"},
			[]get.HTTPRepo{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/3/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"x86_64", "aarch64", "ppc64le", "s390x"},
				},
			},
			false,
			false,
		},
		{
			"Valid maint repo - no valid archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/4/", "SUSE_SLE-15-SP4_Update/",
			[]string{},
			[]get.HTTPRepo{},
			false,
			true,
		},
		{
			"Network error", "http://download.suse.de/ibs/SUSE:/Maintenance:/5/", "SUSE_SLE-15-SP4_Update/",
			[]string{},
			[]get.HTTPRepo{},
			true,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockClient(tt.maint, tt.validArchs, tt.netWorkErr)

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
			"http://download.suse.de/ibs/SUSE:/Maintenance:/6/",
			[]string{"SUSE_SLE-15-SP4_Update/", "SUSE_SLE-15-SP6_Update/"},
			false,
		},
		{
			"Network error",
			"http://download.suse.de/ibs/SUSE:/Maintenance:/7/",
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

func TestGetRepo(t *testing.T) {
	tests := []struct {
		name       string
		mu         string
		validArchs []string
		want       []get.HTTPRepo
		netWorkErr bool
		wantErr    bool
	}{
		{
			"Single arch", "http://download.suse.de/ibs/SUSE:/Maintenance:/8/",
			[]string{"x86_64"},
			[]get.HTTPRepo{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/8/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"x86_64"},
				},
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/8/SUSE_SLE-15-SP6_Update/",
					Archs: []string{"x86_64"},
				},
			},
			false,
			false,
		},
		{
			"Multiple archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/9/",
			[]string{"x86_64", "aarch64", "ppc64le", "s390x"},
			[]get.HTTPRepo{
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/9/SUSE_SLE-15-SP4_Update/",
					Archs: []string{"x86_64", "aarch64", "ppc64le", "s390x"},
				},
				{
					URL:   "http://download.suse.de/ibs/SUSE:/Maintenance:/9/SUSE_SLE-15-SP6_Update/",
					Archs: []string{"x86_64", "aarch64", "ppc64le", "s390x"},
				},
			},
			false,
			false,
		},
		{
			"No available archs", "http://download.suse.de/ibs/SUSE:/Maintenance:/10/",
			[]string{},
			[]get.HTTPRepo{},
			false,
			true,
		},
		{
			"Network error", "http://download.suse.de/ibs/SUSE:/Maintenance:/11/",
			[]string{},
			[]get.HTTPRepo{},
			true,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockClient(tt.mu, tt.validArchs, tt.netWorkErr)

			got, err := GetRepo(client, tt.mu)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.Equal(t, len(tt.want), len(got))

			// to reliably compare expected and got  we need to sort the repos by URL,
			// the results' starting order is influenced by goroutines scheduling
			sort.Slice(got, func(i, j int) bool { return got[i].URL < got[j].URL })
			for i := range tt.want {
				wantRepo := tt.want[i]
				gotRepo := got[i]
				assert.Equal(t, wantRepo.URL, gotRepo.URL)
				assert.ElementsMatch(t, wantRepo.Archs, gotRepo.Archs)
			}
		})
	}
}
