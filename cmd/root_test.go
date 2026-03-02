package cmd

import "testing"

func TestParsePRInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		owner   string
		repo    string
		number  int
	}{
		{
			name:    "short format",
			input:   "owner/repo#123",
			owner:   "owner",
			repo:    "repo",
			number:  123,
			wantErr: false,
		},
		{
			name:    "url format",
			input:   "https://github.com/owner/repo/pull/99",
			owner:   "owner",
			repo:    "repo",
			number:  99,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "owner/repo/123",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePRInput(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got.Owner != tc.owner || got.Repo != tc.repo || got.Number != tc.number {
				t.Fatalf(
					"unexpected parse result: got (%s, %s, %d), want (%s, %s, %d)",
					got.Owner, got.Repo, got.Number, tc.owner, tc.repo, tc.number,
				)
			}
		})
	}
}
