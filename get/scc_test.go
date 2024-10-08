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
		w.Header().Set("Link", "")
		fmt.Fprintf(w, "[{\"url\" : \"http://whatever/SLES15-SP5-Updates\", \"name\" : \"SLES15-SP5-Updates\", \"description\" : \"x86_64 aarch64 s390x ppc64le\"}]")
	})

	tests := []struct {
		name    string
		user    string
		pass    string
		names   []string
		archs   []string
		want    []HTTPRepoConfig
		wantErr bool
	}{
		{
			"One name and no matching arch", "user", "pass",
			[]string{"SLES15-SP5-Pool"}, []string{"s390x"},
			[]HTTPRepoConfig{},
			false,
		},
		{
			"One name and one matching arch", "user", "pass",
			[]string{"SLES15-SP5-Pool"}, []string{"x86_64"},
			[]HTTPRepoConfig{
				{URL: "http://whatever/SLES15-SP5-Pool", Archs: []string{"x86_64"}},
			},
			false,
		},
		{
			"One name and multiple matching archs", "user", "pass",
			[]string{"SLES15-SP5-Pool"}, []string{"aarch64", "i586"},
			[]HTTPRepoConfig{
				{URL: "http://whatever/SLES15-SP5-Pool", Archs: []string{"aarch64", "i586"}},
			},
			false,
		},
		{
			"Multiple names and no matching archs", "user", "pass",
			[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"src"},
			[]HTTPRepoConfig{},
			false,
		},
		{
			"Multiple names and multiple matching archs", "user", "pass",
			[]string{"SLES15-SP5-Pool", "SLES15-SP5-Updates"}, []string{"x86_64", "aarch64"},
			[]HTTPRepoConfig{
				{URL: "http://whatever/SLES15-SP5-Pool", Archs: []string{"x86_64", "aarch64"}},
				{URL: "http://whatever/SLES15-SP5-Updates", Archs: []string{"x86_64", "aarch64"}},
			},
			false,
		},
		{
			"Invalid user", "thiswillfail", "pass",
			[]string{"SLES15-SP5-Pool"}, []string{"x86_64"},
			[]HTTPRepoConfig{},
			true,
		},
		{
			"Invalid password", "user", "thiswillfail",
			[]string{"SLES15-SP5-Pool"}, []string{"x86_64"},
			[]HTTPRepoConfig{},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpConfigs, err := SCCToHTTPConfigs("http://localhost:8080", tt.user, tt.pass, []SCCReposConfig{
				{
					Names: tt.names,
					Archs: tt.archs,
				},
			})
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.Equal(t, len(tt.want), len(httpConfigs))

			for i := range httpConfigs {
				wantConfig := tt.want[i]
				gotConfig := httpConfigs[i]

				assert.EqualValues(t, wantConfig.URL, gotConfig.URL)
				assert.ElementsMatch(t, wantConfig.Archs, gotConfig.Archs)
			}
		})
	}
}
