package github

import (
	"testing"

	"github.com/drpaneas/prview/internal/model"
)

func TestBasePathForDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		diff model.FileDiff
		want string
	}{
		{
			name: "renamed file uses old path",
			diff: model.FileDiff{
				Path:    "new/path.go",
				OldPath: "old/path.go",
				Status:  "renamed",
			},
			want: "old/path.go",
		},
		{
			name: "renamed file without old path uses current path",
			diff: model.FileDiff{
				Path:   "new/path.go",
				Status: "renamed",
			},
			want: "new/path.go",
		},
		{
			name: "modified file uses current path",
			diff: model.FileDiff{
				Path:   "pkg/file.go",
				Status: "modified",
			},
			want: "pkg/file.go",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := basePathForDiff(tc.diff)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
