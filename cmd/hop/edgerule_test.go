package main

import (
	"testing"
)

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name   string
		urlStr string
		want   bool
	}{
		{
			name:   "valid HTTP URL",
			urlStr: "http://example.com",
			want:   true,
		},
		{
			name:   "valid HTTPS URL",
			urlStr: "https://example.com",
			want:   true,
		},
		{
			name:   "valid URL with path",
			urlStr: "https://example.com/path/to/resource",
			want:   true,
		},
		{
			name:   "valid URL with subdomain",
			urlStr: "https://api.example.com",
			want:   true,
		},
		{
			name:   "invalid URL - malformed",
			urlStr: "not-a-url",
			want:   false,
		},
		{
			name:   "invalid URL - no host",
			urlStr: "/path/only",
			want:   false,
		},
		{
			name:   "empty string",
			urlStr: "",
			want:   false,
		},
		{
			name:   "URL with port",
			urlStr: "https://example.com:8080",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDomain(tt.urlStr)
			if got != tt.want {
				t.Errorf("isValidDomain(%q) = %v, want %v", tt.urlStr, got, tt.want)
			}
		})
	}
}

func TestIsSuspiciousURL(t *testing.T) {
	tests := []struct {
		name       string
		urlStr     string
		wantFlag   bool
		wantReason string
	}{
		{
			name:       "legitimate URL",
			urlStr:     "https://example.com/page",
			wantFlag:   false,
			wantReason: "",
		},
		{
			name:       "bit.ly shortener",
			urlStr:     "https://bit.ly/abc123",
			wantFlag:   true,
			wantReason: "URL shortener detected",
		},
		{
			name:       "tinyurl shortener",
			urlStr:     "https://tinyurl.com/abc123",
			wantFlag:   true,
			wantReason: "URL shortener detected",
		},
		{
			name:       "IP address instead of domain",
			urlStr:     "https://192.168.1.1/page",
			wantFlag:   true,
			wantReason: "IP address instead of domain",
		},
		{
			name:       "suspicious heroku pattern",
			urlStr:     "https://abc-def-ghi.herokuapp.com",
			wantFlag:   true,
			wantReason: "Suspicious Heroku subdomain pattern",
		},
		{
			name:       "long random domain",
			urlStr:     "https://abcdefghijklmnopqrstuvwxyz.com",
			wantFlag:   true,
			wantReason: "Suspiciously long random domain",
		},
		{
			name:       "phishing keyword",
			urlStr:     "https://phishing-site.com",
			wantFlag:   true,
			wantReason: "Contains suspicious keywords",
		},
		{
			name:       "malware keyword",
			urlStr:     "https://malware-download.com",
			wantFlag:   true,
			wantReason: "Contains suspicious keywords",
		},
		{
			name:       "legitimate long domain",
			urlStr:     "https://www.verylongcompanyname.com",
			wantFlag:   false,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFlag, gotReason := isSuspiciousURL(tt.urlStr)
			if gotFlag != tt.wantFlag {
				t.Errorf("isSuspiciousURL(%q) flag = %v, want %v", tt.urlStr, gotFlag, tt.wantFlag)
			}
			if gotReason != tt.wantReason {
				t.Errorf("isSuspiciousURL(%q) reason = %q, want %q", tt.urlStr, gotReason, tt.wantReason)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name   string
		urlStr string
		want   string
	}{
		{
			name:   "uppercase to lowercase",
			urlStr: "HTTPS://EXAMPLE.COM/PATH",
			want:   "https://example.com/path",
		},
		{
			name:   "remove trailing slash",
			urlStr: "https://example.com/path/",
			want:   "https://example.com/path",
		},
		{
			name:   "keep root slash",
			urlStr: "/",
			want:   "/",
		},
		{
			name:   "already normalized",
			urlStr: "https://example.com/path",
			want:   "https://example.com/path",
		},
		{
			name:   "mixed case with trailing slash",
			urlStr: "HTTPS://Example.Com/Path/",
			want:   "https://example.com/path",
		},
		{
			name:   "empty string",
			urlStr: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeURL(tt.urlStr)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.urlStr, got, tt.want)
			}
		})
	}
}

func TestExtractSourceURL(t *testing.T) {
	tests := []struct {
		name string
		rule EdgeRuleResponse
		want string
	}{
		{
			name: "rule with trigger and pattern match",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{
					{
						PatternMatches: []string{"/old-path", "/another-path"},
					},
				},
			},
			want: "/old-path",
		},
		{
			name: "rule with no triggers",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{},
			},
			want: "",
		},
		{
			name: "rule with trigger but no pattern matches",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{
					{
						PatternMatches: []string{},
					},
				},
			},
			want: "",
		},
		{
			name: "rule with nil triggers",
			rule: EdgeRuleResponse{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSourceURL(tt.rule)
			if got != tt.want {
				t.Errorf("extractSourceURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRedirectMap(t *testing.T) {
	tests := []struct {
		name  string
		rules []EdgeRuleResponse
		want  *RedirectMap
	}{
		{
			name: "single redirect rule",
			rules: []EdgeRuleResponse{
				{
					ActionType:       1,
					ActionParameter1: "https://newsite.com",
					Triggers: []Trigger{
						{
							PatternMatches: []string{"/old-path"},
						},
					},
				},
			},
			want: &RedirectMap{
				SourceToDestination: map[string]string{
					"/old-path": "https://newsite.com",
				},
				Rules: map[string]*EdgeRuleResponse{
					"/old-path": {
						ActionType:       1,
						ActionParameter1: "https://newsite.com",
						Triggers: []Trigger{
							{
								PatternMatches: []string{"/old-path"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple redirect rules",
			rules: []EdgeRuleResponse{
				{
					ActionType:       1,
					ActionParameter1: "https://newsite.com/page1",
					Triggers: []Trigger{
						{
							PatternMatches: []string{"/old-path1"},
						},
					},
				},
				{
					ActionType:       1,
					ActionParameter1: "https://newsite.com/page2",
					Triggers: []Trigger{
						{
							PatternMatches: []string{"/old-path2"},
						},
					},
				},
			},
			want: &RedirectMap{
				SourceToDestination: map[string]string{
					"/old-path1": "https://newsite.com/page1",
					"/old-path2": "https://newsite.com/page2",
				},
				Rules: map[string]*EdgeRuleResponse{
					"/old-path1": {
						ActionType:       1,
						ActionParameter1: "https://newsite.com/page1",
						Triggers: []Trigger{
							{
								PatternMatches: []string{"/old-path1"},
							},
						},
					},
					"/old-path2": {
						ActionType:       1,
						ActionParameter1: "https://newsite.com/page2",
						Triggers: []Trigger{
							{
								PatternMatches: []string{"/old-path2"},
							},
						},
					},
				},
			},
		},
		{
			name: "non-redirect rules ignored",
			rules: []EdgeRuleResponse{
				{
					ActionType:       2, // Not a redirect
					ActionParameter1: "https://newsite.com",
					Triggers: []Trigger{
						{
							PatternMatches: []string{"/old-path"},
						},
					},
				},
			},
			want: &RedirectMap{
				SourceToDestination: map[string]string{},
				Rules:               map[string]*EdgeRuleResponse{},
			},
		},
		{
			name: "redirect without destination ignored",
			rules: []EdgeRuleResponse{
				{
					ActionType:       1,
					ActionParameter1: "", // Empty destination
					Triggers: []Trigger{
						{
							PatternMatches: []string{"/old-path"},
						},
					},
				},
			},
			want: &RedirectMap{
				SourceToDestination: map[string]string{},
				Rules:               map[string]*EdgeRuleResponse{},
			},
		},
		{
			name:  "empty rules",
			rules: []EdgeRuleResponse{},
			want: &RedirectMap{
				SourceToDestination: map[string]string{},
				Rules:               map[string]*EdgeRuleResponse{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRedirectMap(tt.rules)

			// Compare SourceToDestination maps
			if len(got.SourceToDestination) != len(tt.want.SourceToDestination) {
				t.Errorf("buildRedirectMap() SourceToDestination length = %d, want %d",
					len(got.SourceToDestination), len(tt.want.SourceToDestination))
				return
			}

			for key, wantValue := range tt.want.SourceToDestination {
				if gotValue, exists := got.SourceToDestination[key]; !exists || gotValue != wantValue {
					t.Errorf("buildRedirectMap() SourceToDestination[%q] = %q, want %q", key, gotValue, wantValue)
				}
			}

			// Compare Rules maps length
			if len(got.Rules) != len(tt.want.Rules) {
				t.Errorf("buildRedirectMap() Rules length = %d, want %d",
					len(got.Rules), len(tt.want.Rules))
			}
		})
	}
}
