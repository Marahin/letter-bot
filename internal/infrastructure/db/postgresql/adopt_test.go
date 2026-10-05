package postgresql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdoptionVersions(t *testing.T) {
	local := []int64{20240429143025, 20240429143026, 20250611195746, 20251211123500, 20260926100000}
	executed := func(v string) atlasRevision {
		return atlasRevision{Version: v, Type: atlasExecute, Applied: 3, Total: 3}
	}

	tests := []struct {
		name    string
		revs    []atlasRevision
		want    []int64
		wantErr string
	}{
		{
			name: "a baseline covers every earlier version",
			revs: []atlasRevision{{Version: "20240429143026", Type: atlasBaseline}},
			want: []int64{20240429143025, 20240429143026},
		},
		{
			name: "a baseline and executed revisions",
			revs: []atlasRevision{{Version: "20240429143026", Type: atlasBaseline}, executed("20250611195746"), executed("20251211123500")},
			want: []int64{20240429143025, 20240429143026, 20250611195746, 20251211123500},
		},
		{
			name: "a resolved revision is done",
			revs: []atlasRevision{executed("20240429143025"), {Version: "20240429143026", Type: atlasExecute | atlasResolved, Applied: 1, Total: 4, Error: "boom"}},
			want: []int64{20240429143025, 20240429143026},
		},
		{
			name: "non-numeric rows are ignored",
			revs: []atlasRevision{{Version: ".atlas_cloud_identifiers"}, executed("20240429143025")},
			want: []int64{20240429143025},
		},
		{
			name:    "a partial revision stops the adoption",
			revs:    []atlasRevision{executed("20240429143025"), {Version: "20240429143026", Type: atlasExecute, Applied: 2, Total: 5}},
			wantErr: "atlas revision 20240429143026 is partially applied (2/5",
		},
		{
			name:    "a failed revision stops the adoption",
			revs:    []atlasRevision{{Version: "20240429143025", Type: atlasExecute, Applied: 3, Total: 3, Error: "boom"}},
			wantErr: "atlas revision 20240429143025 is partially applied",
		},
		{
			name:    "a database ahead of the binary",
			revs:    []atlasRevision{executed("20240429143025"), executed("20990101000000")},
			wantErr: "atlas revision 20990101000000 is not a migration of this binary",
		},
		{
			name: "an empty history",
			revs: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := adoptionVersions(tt.revs, local)

			// then
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
