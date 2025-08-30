package main

import (
	"encoding/json"
	"strings"
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

func TestFormatBoolStatus(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected string
	}{
		{
			name:     "enabled status",
			enabled:  true,
			expected: "Enabled",
		},
		{
			name:     "disabled status",
			enabled:  false,
			expected: "Disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBoolStatus(tt.enabled)
			if result != tt.expected {
				t.Errorf("formatBoolStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatSSLCertificateStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected string
	}{
		{
			name:     "not configured status",
			status:   0,
			expected: "Not configured",
		},
		{
			name:     "pending status",
			status:   1,
			expected: "Pending",
		},
		{
			name:     "active status",
			status:   2,
			expected: "Active",
		},
		{
			name:     "failed status",
			status:   3,
			expected: "Failed",
		},
		{
			name:     "expired status",
			status:   4,
			expected: "Expired",
		},
		{
			name:     "unknown status",
			status:   99,
			expected: "Unknown (99)",
		},
		{
			name:     "negative status",
			status:   -1,
			expected: "Unknown (-1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSSLCertificateStatus(tt.status)
			if result != tt.expected {
				t.Errorf("formatSSLCertificateStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStrictUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid JSON matching struct",
			jsonData:    `{"Id": 123, "Name": "test", "EdgeRules": [], "Hostnames": []}`,
			expectError: false,
		},
		{
			name:        "JSON with extra field - should be allowed",
			jsonData:    `{"Id": 123, "Name": "test", "EdgeRules": [], "Hostnames": [], "ExtraField": "value"}`,
			expectError: false, // Extra API fields are now OK
		},
		{
			name:        "JSON missing field that struct expects",
			jsonData:    `{"Name": "test", "EdgeRules": [], "Hostnames": []}`,
			expectError: true, // Missing API fields that struct expects should fail
			errorMsg:    "struct expects field 'Id'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pullZone PullZoneDetails
			err := strictUnmarshal([]byte(tt.jsonData), &pullZone)

			if tt.expectError {
				if err == nil {
					t.Errorf("strictUnmarshal() expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("strictUnmarshal() error = %v, expected to contain %s", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("strictUnmarshal() unexpected error: %v", err)
				}
			}
		})
	}
}
