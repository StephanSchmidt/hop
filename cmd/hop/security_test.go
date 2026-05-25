package main

import (
	"testing"
)

func TestAnalyzeSecurityHeaders(t *testing.T) {
	tests := []struct {
		name             string
		rules            []EdgeRuleResponse
		wantSuccessCount int
		wantIssueCount   int
		wantErrorCount   int // Issues with severity "error"
		wantWarningCount int // Issues with severity "warning"
	}{
		{
			name:             "no rules configured",
			rules:            []EdgeRuleResponse{},
			wantSuccessCount: 0,
			wantIssueCount:   6, // All 6 headers missing
			wantErrorCount:   3, // 3 critical headers
			wantWarningCount: 3, // 3 recommended headers
		},
		{
			name: "all critical headers configured",
			rules: []EdgeRuleResponse{
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "Strict-Transport-Security",
					ActionParameter2: "max-age=63072000; includeSubDomains",
					Enabled:          true,
				},
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "X-Frame-Options",
					ActionParameter2: "DENY",
					Enabled:          true,
				},
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "X-Content-Type-Options",
					ActionParameter2: "nosniff",
					Enabled:          true,
				},
			},
			wantSuccessCount: 3,
			wantIssueCount:   3, // 3 recommended headers missing
			wantErrorCount:   0,
			wantWarningCount: 3,
		},
		{
			name: "all headers configured",
			rules: []EdgeRuleResponse{
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "Strict-Transport-Security", ActionParameter2: "max-age=63072000", Enabled: true},
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "X-Frame-Options", ActionParameter2: "DENY", Enabled: true},
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "X-Content-Type-Options", ActionParameter2: "nosniff", Enabled: true},
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "Referrer-Policy", ActionParameter2: "strict-origin-when-cross-origin", Enabled: true},
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "X-XSS-Protection", ActionParameter2: "0", Enabled: true},
				{ActionType: ActionTypeSetResponseHeader, ActionParameter1: "Permissions-Policy", ActionParameter2: "geolocation=()", Enabled: true},
			},
			wantSuccessCount: 6,
			wantIssueCount:   0,
			wantErrorCount:   0,
			wantWarningCount: 0,
		},
		{
			name: "disabled rules ignored",
			rules: []EdgeRuleResponse{
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "Strict-Transport-Security",
					ActionParameter2: "max-age=63072000",
					Enabled:          false, // Disabled
				},
			},
			wantSuccessCount: 0,
			wantIssueCount:   6,
			wantErrorCount:   3,
			wantWarningCount: 3,
		},
		{
			name: "non-header rules ignored",
			rules: []EdgeRuleResponse{
				{
					ActionType:       ActionTypeRedirect, // Not a header rule
					ActionParameter1: "https://example.com",
					ActionParameter2: "302",
					Enabled:          true,
				},
			},
			wantSuccessCount: 0,
			wantIssueCount:   6,
			wantErrorCount:   3,
			wantWarningCount: 3,
		},
		{
			name: "case insensitive header matching",
			rules: []EdgeRuleResponse{
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "strict-transport-security", // lowercase
					ActionParameter2: "max-age=63072000",
					Enabled:          true,
				},
				{
					ActionType:       ActionTypeSetResponseHeader,
					ActionParameter1: "X-FRAME-OPTIONS", // uppercase
					ActionParameter2: "DENY",
					Enabled:          true,
				},
			},
			wantSuccessCount: 2,
			wantIssueCount:   4,
			wantErrorCount:   1, // Only X-Content-Type-Options missing
			wantWarningCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeSecurityHeaders(tt.rules)

			if len(result.Successful) != tt.wantSuccessCount {
				t.Errorf("analyzeSecurityHeaders() success count = %d, want %d", len(result.Successful), tt.wantSuccessCount)
			}

			if len(result.Issues) != tt.wantIssueCount {
				t.Errorf("analyzeSecurityHeaders() issue count = %d, want %d", len(result.Issues), tt.wantIssueCount)
			}

			// Count errors and warnings
			errorCount := 0
			warningCount := 0
			for _, issue := range result.Issues {
				switch issue.Severity {
				case "error":
					errorCount++
				case "warning":
					warningCount++
				}
			}

			if errorCount != tt.wantErrorCount {
				t.Errorf("analyzeSecurityHeaders() error count = %d, want %d", errorCount, tt.wantErrorCount)
			}

			if warningCount != tt.wantWarningCount {
				t.Errorf("analyzeSecurityHeaders() warning count = %d, want %d", warningCount, tt.wantWarningCount)
			}
		})
	}
}

func TestActionTypeConstants(t *testing.T) {
	// Verify the action type constants match Bunny CDN API
	tests := []struct {
		name       string
		actionType int
		want       int
	}{
		{"ForceSSL", ActionTypeForceSSL, 0},
		{"Redirect", ActionTypeRedirect, 1},
		{"OriginURL", ActionTypeOriginURL, 2},
		{"OverrideCacheTime", ActionTypeOverrideCacheTime, 3},
		{"BlockRequest", ActionTypeBlockRequest, 4},
		{"SetResponseHeader", ActionTypeSetResponseHeader, 5},
		{"SetRequestHeader", ActionTypeSetRequestHeader, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actionType != tt.want {
				t.Errorf("ActionType%s = %d, want %d", tt.name, tt.actionType, tt.want)
			}
		})
	}
}

func TestRecommendedHeadersCount(t *testing.T) {
	// Verify we have the expected number of recommended headers
	// Note: COOP, COEP, CORP omitted (break third-party widgets)
	if len(recommendedHeaders) != 6 {
		t.Errorf("recommendedHeaders count = %d, want 6", len(recommendedHeaders))
	}

	// Verify critical headers count (should be 3)
	criticalCount := 0
	for _, h := range recommendedHeaders {
		if h.Severity == "error" {
			criticalCount++
		}
	}
	if criticalCount != 3 {
		t.Errorf("critical headers count = %d, want 3", criticalCount)
	}
}

func TestRecommendedHeadersContent(t *testing.T) {
	// Verify specific critical headers are present
	criticalHeaders := []string{
		"Strict-Transport-Security",
		"X-Frame-Options",
		"X-Content-Type-Options",
	}

	for _, criticalHeader := range criticalHeaders {
		found := false
		for _, h := range recommendedHeaders {
			if h.Name == criticalHeader && h.Severity == "error" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("critical header %q not found or not marked as error severity", criticalHeader)
		}
	}
}
