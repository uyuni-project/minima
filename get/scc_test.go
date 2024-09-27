package get

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSCCToHTTPConfigs(t *testing.T) {
	expectedToken := base64.URLEncoding.EncodeToString([]byte("user:pass"))
	expectedAuth := "Basic " + expectedToken

	http.HandleFunc("/connect/organizations/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedAuth {
			w.WriteHeader(401)
			return
		}

		w.Header().Set("Link", "<http://localhost:8080/connect/organizations/repositories2>; rel=\"next\"")
		fmt.Fprintf(w, "[{\"url\" : \"http://whatever/SLES15-SP5-Pool\", \"name\" : \"SLES15-SP5-Pool\", \"description\" : \"x86_64 aarch64 i586\"}]")
	})

	http.HandleFunc("/connect/organizations/repositories2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "<http://localhost:8080/connect/organizations/repositories3>; rel=\"next\"")
		fmt.Fprintf(w, "[{\"url\" : \"http://whatever/SLES15-SP5-Updates\", \"name\" : \"SLES15-SP5-Updates\", \"description\" : \"x86_64 aarch64 s390x ppc64le\"}]")
	})

	http.HandleFunc("/connect/organizations/repositories3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "")
		fmt.Fprintf(w, "[{\"url\" : \"http://whatever/SLES15-SP6-Pool\", \"name\" : \"SLES15-SP6-Pool\", \"description\" : \"x86_64 aarch64\"}]")
	})

	tests := []struct {
		name     string
		user     string
		pass     string
		sccInput []SCCReposConfig
		want     []HTTPReposConfig
		wantErr  bool
	}{
		{
			"One name and no matching arch", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool"}, []string{"s390x"}},
			},
			[]HTTPReposConfig{},
			false,
		},
		{
			"One name and one matching arch", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP5-Pool"},
					Archs: []string{"x86_64"},
				},
			},
			false,
		},
		{
			"One name and multiple matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool"}, []string{"aarch64", "i586"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP5-Pool"},
					Archs: []string{"aarch64", "i586"},
				},
			},
			false,
		},
		{
			"Multiple names and no matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"src"}},
			},
			[]HTTPReposConfig{},
			false,
		},
		{
			"Multiple names and multiple matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"x86_64", "aarch64"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP5-Pool", "http://whatever/SLES15-SP5-Updates"},
					Archs: []string{"x86_64", "aarch64"},
				},
			},
			false,
		},
		{
			"Multiple configs and no matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"src"}},
				{[]string{"SLES15-SP6-Pool"}, []string{"s390x"}},
			},
			[]HTTPReposConfig{},
			false,
		},
		{
			"Multiple configs and one matching arch in the 2nd", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"src"}},
				{[]string{"SLES15-SP6-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP6-Pool"},
					Archs: []string{"x86_64"},
				},
			},
			false,
		},
		{
			"Multiple configs and multiple matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"x86_64"}},
				{[]string{"SLES15-SP6-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP5-Pool", "http://whatever/SLES15-SP5-Updates"},
					Archs: []string{"x86_64"},
				},
				{
					URLs:  []string{"http://whatever/SLES15-SP6-Pool"},
					Archs: []string{"x86_64"},
				},
			},
			false,
		},
		{
			"Multiple configs and partially matching archs", "user", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"s390x"}},
				{[]string{"SLES15-SP6-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{
				{
					URLs:  []string{"http://whatever/SLES15-SP5-Updates"},
					Archs: []string{"s390x"},
				},
				{
					URLs:  []string{"http://whatever/SLES15-SP6-Pool"},
					Archs: []string{"x86_64"},
				},
			},
			false,
		},
		{
			"Invalid user", "thiswillfail", "pass",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{},
			true,
		},
		{
			"Invalid password", "user", "thiswillfail",
			[]SCCReposConfig{
				{[]string{"SLES15-SP5-Pool"}, []string{"x86_64"}},
			},
			[]HTTPReposConfig{},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpConfigs, err := SCCToHTTPConfigs("http://localhost:8080", tt.user, tt.pass, tt.sccInput)

			fmt.Println(httpConfigs)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.Equal(t, len(tt.want), len(httpConfigs))

			for i := range httpConfigs {
				wantConfig := tt.want[i]
				gotConfig := httpConfigs[i]

				assert.ElementsMatch(t, wantConfig.URLs, gotConfig.URLs)
				assert.ElementsMatch(t, wantConfig.Archs, gotConfig.Archs)
			}
		})
	}
}
