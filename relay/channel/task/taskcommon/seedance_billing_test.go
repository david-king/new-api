package taskcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedance2PriceRatio(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		resolution    string
		hasVideoInput bool
		want          float64
	}{
		{
			name:  "zlhub 2.0 base",
			model: "doubao-seedance-2.0",
			want:  1,
		},
		{
			name:       "zlhub 2.0 1080p",
			model:      "doubao-seedance-2.0",
			resolution: "1080p",
			want:       51.0 / 46.0,
		},
		{
			name:          "zlhub 2.0 video input",
			model:         "doubao-seedance-2.0",
			hasVideoInput: true,
			want:          28.0 / 46.0,
		},
		{
			name:          "zlhub 2.0 1080p video input",
			model:         "doubao-seedance-2.0",
			resolution:    "1080p",
			hasVideoInput: true,
			want:          31.0 / 46.0,
		},
		{
			name:          "doubao 2.0 fast video input",
			model:         "doubao-seedance-2-0-fast-260128",
			hasVideoInput: true,
			want:          22.0 / 37.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Seedance2PriceRatio(tt.model, tt.resolution, tt.hasVideoInput)
			require.True(t, ok)
			assert.InDelta(t, tt.want, got, 0.000001)
		})
	}
}
