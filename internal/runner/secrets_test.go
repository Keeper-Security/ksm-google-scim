package runner

import (
	"context"
	"reflect"
	"testing"
)

func TestGCPSecretResourceName(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "full path with version is unchanged",
			ref:  "projects/my-proj/secrets/scim-token/versions/3",
			want: "projects/my-proj/secrets/scim-token/versions/3",
		},
		{
			name: "path without version defaults to latest",
			ref:  "projects/my-proj/secrets/scim-token",
			want: "projects/my-proj/secrets/scim-token/versions/latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gcpSecretResourceName(context.Background(), tt.ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseScimGroupList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "comma separated", raw: "Group A,Group B", want: []string{"Group A", "Group B"}},
		{name: "trims whitespace", raw: " Group A , Group B ", want: []string{"Group A", "Group B"}},
		{name: "newline separated", raw: "Group A\nGroup B\n", want: []string{"Group A", "Group B"}},
		{name: "drops empty entries", raw: "Group A,,Group B,", want: []string{"Group A", "Group B"}},
		{name: "single", raw: "Group A", want: []string{"Group A"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseScimGroupList(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
