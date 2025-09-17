package restic_test

import (
	"testing"

	restic "incus-backup/src/restic"
)

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard output",
			input: "restic 0.18.2 compiled with go1.22.0 on linux/amd64\n",
			want:  "0.18.2",
		},
		{
			name:  "prerelease",
			input: "restic 0.18.0-dev (compiled manually)\n",
			want:  "0.18.0-dev",
		},
		{
			name:    "no match",
			input:   "restic version output is unexpected\n",
			want:    "",
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := restic.ExtractVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsCompatible(t *testing.T) {
	if !restic.IsCompatible("0.18.0") {
		t.Fatalf("expected 0.18.0 to be compatible")
	}
	if !restic.IsCompatible("0.19.1") {
		t.Fatalf("expected newer version to be compatible")
	}
	if restic.IsCompatible("0.17.9") {
		t.Fatalf("expected older version to be incompatible")
	}
	if restic.IsCompatible("") {
		t.Fatalf("expected empty version to be incompatible")
	}
}
