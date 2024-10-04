package maint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToIncidentNumberSet(t *testing.T) {
	tests := []struct {
		name string
		ups  []Updates
		want stringSet
	}{
		{
			"No duplicates",
			[]Updates{{IncidentNumber: "1"}, {IncidentNumber: "2"}, {IncidentNumber: "3"}},
			map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			},
		},
		{
			"Duplicates",
			[]Updates{{IncidentNumber: "1"}, {IncidentNumber: "1"}, {IncidentNumber: "2"}, {IncidentNumber: "2"}, {IncidentNumber: "3"}},
			map[string]struct{}{
				"1": {},
				"2": {},
				"3": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toIncidentNumberSet(tt.ups)
			assert.Equal(t, len(tt.want), len(got))
			for k := range tt.want {
				_, ok := got[k]
				if !ok {
					t.Errorf("%s not found in results", k)
				}
			}
		})
	}
}

func TestCleanWebChunks(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{
			"All valid",
			[]string{">SUSE_SLE-15-SP4_Update/<", ">SUSE_SLE-15-SP6_Pool/<", ">openSUSE_Leap-15.5/<"},
			[]string{"SUSE_SLE-15-SP4_Update/", "SUSE_SLE-15-SP6_Pool/", "openSUSE_Leap-15.5/"},
		},
		{
			"Some junk",
			[]string{"SUSE ", ">SUSE_SLE-15-SP4_Update/<", ">SUSE_SLE-15-SP6_Pool/<", ">openSUSE_Leap-15.5/<", "=\"SUSE_SLE-15-SP6_Pool\""},
			[]string{"SUSE_SLE-15-SP4_Update/", "SUSE_SLE-15-SP6_Pool/", "openSUSE_Leap-15.5/"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanWebChunks(tt.chunks)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
