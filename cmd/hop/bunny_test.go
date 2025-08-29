package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBunnyTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantTime  time.Time
		wantError bool
	}{
		{
			name:      "Bunny CDN format",
			input:     []byte(`"2025-08-29T11:10:09.594"`),
			wantTime:  time.Date(2025, 8, 29, 11, 10, 9, 594000000, time.UTC),
			wantError: false,
		},
		{
			name:      "Bunny CDN format without milliseconds",
			input:     []byte(`"2025-08-29T11:10:09"`),
			wantTime:  time.Date(2025, 8, 29, 11, 10, 9, 0, time.UTC),
			wantError: false,
		},
		{
			name:      "RFC3339 format fallback",
			input:     []byte(`"2025-08-29T11:10:09Z"`),
			wantTime:  time.Date(2025, 8, 29, 11, 10, 9, 0, time.UTC),
			wantError: false,
		},
		{
			name:      "RFC3339 with timezone",
			input:     []byte(`"2025-08-29T11:10:09+02:00"`),
			wantTime:  time.Date(2025, 8, 29, 11, 10, 9, 0, time.FixedZone("", 2*3600)),
			wantError: false,
		},
		{
			name:      "null value",
			input:     []byte(`"null"`),
			wantTime:  time.Time{},
			wantError: false,
		},
		{
			name:      "empty string",
			input:     []byte(`""`),
			wantTime:  time.Time{},
			wantError: false,
		},
		{
			name:      "actual JSON null",
			input:     []byte(`null`),
			wantTime:  time.Time{},
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     []byte(`"invalid-time-format"`),
			wantTime:  time.Time{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bt BunnyTime
			err := json.Unmarshal(tt.input, &bt)

			if tt.wantError {
				if err == nil {
					t.Errorf("BunnyTime.UnmarshalJSON() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("BunnyTime.UnmarshalJSON() unexpected error: %v", err)
				return
			}

			if !bt.Time.Equal(tt.wantTime) {
				t.Errorf("BunnyTime.UnmarshalJSON() time = %v, want %v", bt.Time, tt.wantTime)
			}
		})
	}
}
