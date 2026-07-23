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
		{
			name: "extension trigger is not a source",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{
					{
						Type:           TriggerTypeURLExtension,
						PatternMatches: []string{"{{empty}}"},
					},
				},
			},
			want: "",
		},
		{
			name: "negated URL trigger is not a source",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{
					{
						Type:                TriggerTypeURL,
						PatternMatches:      []string{"*/"},
						PatternMatchingType: PatternMatchingNone,
					},
				},
			},
			want: "",
		},
		{
			name: "positive URL trigger found after non-URL trigger",
			rule: EdgeRuleResponse{
				Triggers: []Trigger{
					{
						Type:           TriggerTypeURLExtension,
						PatternMatches: []string{"{{empty}}"},
					},
					{
						Type:           TriggerTypeURL,
						PatternMatches: []string{"/old-path"},
					},
				},
			},
			want: "/old-path",
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

// slashAddTestRules mirrors the two rules created by createSlashAddRules,
// as they come back from the Bunny API
func slashAddTestRules() []EdgeRuleResponse {
	makeRule := func(desc, dest string, queryMatching int) EdgeRuleResponse {
		return EdgeRuleResponse{
			ActionType:          ActionTypeRedirect,
			ActionParameter1:    dest,
			ActionParameter2:    "302",
			TriggerMatchingType: TriggerMatchingAll,
			Description:         desc,
			Enabled:             true,
			Triggers: []Trigger{
				{Type: TriggerTypeURLExtension, PatternMatches: []string{"{{empty}}"}, PatternMatchingType: PatternMatchingAny},
				{Type: TriggerTypeURL, PatternMatches: []string{"*/"}, PatternMatchingType: PatternMatchingNone},
				{Type: TriggerTypeUrlQueryString, PatternMatches: []string{"?*=*"}, PatternMatchingType: queryMatching},
			},
		}
	}
	return []EdgeRuleResponse{
		makeRule(SlashAddDescNoQuery, "https://www.example.com%{Url.Directory}%{Url.FileName}/", PatternMatchingNone),
		makeRule(SlashAddDescWithQuery, "https://www.example.com%{Url.Directory}%{Url.FileName}/?%{Request.QueryString}", PatternMatchingAny),
	}
}

func TestSlashRulesNotFlaggedAsDuplicates(t *testing.T) {
	issues := checkConfigurationIssues(slashAddTestRules())
	if len(issues) != 0 {
		t.Errorf("checkConfigurationIssues() flagged slash rules: %+v", issues)
	}
}

func TestSlashRulesPassBasicChecks(t *testing.T) {
	issues := checkBasicRedirectIssues(slashAddTestRules())
	if len(issues) != 0 {
		t.Errorf("checkBasicRedirectIssues() flagged slash rules: %+v", issues)
	}
}

func Test301AlwaysFlagged(t *testing.T) {
	rules := []EdgeRuleResponse{
		{
			ActionType:       ActionTypeRedirect,
			ActionParameter1: "https://www.example.com/new/",
			ActionParameter2: "301",
			Description:      "Manual 301 redirect",
			Triggers: []Trigger{
				{Type: TriggerTypeURL, PatternMatches: []string{"/old"}},
			},
		},
		{
			ActionType:       ActionTypeRedirect,
			ActionParameter1: "https://www.example.com%{Url.Directory}%{Url.FileName}/",
			ActionParameter2: "301",
			Description:      SlashAddDescNoQuery,
			Triggers: []Trigger{
				{Type: TriggerTypeURLExtension, PatternMatches: []string{"{{empty}}"}},
			},
		},
	}
	issues := checkBasicRedirectIssues(rules)
	if len(issues) != 2 {
		t.Fatalf("checkBasicRedirectIssues() returned %d issues, want 2 (no rule is exempt from the 301 warning)", len(issues))
	}
}

func TestSlashRulesExcludedFromRedirectMap(t *testing.T) {
	rm := buildRedirectMap(slashAddTestRules())
	if len(rm.SourceToDestination) != 0 {
		t.Errorf("buildRedirectMap() included slash rules: %+v", rm.SourceToDestination)
	}
}

func TestHasBunnyVariables(t *testing.T) {
	if !hasBunnyVariables("https://www.example.com%{Url.Directory}%{Url.FileName}/") {
		t.Error("hasBunnyVariables() = false for templated URL, want true")
	}
	if hasBunnyVariables("https://www.example.com/page/") {
		t.Error("hasBunnyVariables() = true for plain URL, want false")
	}
}

func TestSlashAddTriggerConstruction(t *testing.T) {
	// Test that redirect URLs are constructed correctly
	host := "example.com"
	wantNoQuery := "https://example.com%{Url.Directory}%{Url.FileName}/"
	wantWithQuery := "https://example.com%{Url.Directory}%{Url.FileName}/?%{Request.QueryString}"

	noQueryURL := "https://" + host + "%{Url.Directory}%{Url.FileName}/"
	if noQueryURL != wantNoQuery {
		t.Errorf("no query URL = %s, want %s", noQueryURL, wantNoQuery)
	}

	withQueryURL := "https://" + host + "%{Url.Directory}%{Url.FileName}/?%{Request.QueryString}"
	if withQueryURL != wantWithQuery {
		t.Errorf("with query URL = %s, want %s", withQueryURL, wantWithQuery)
	}
}

func TestSlashRuleIDTags(t *testing.T) {
	// Verify the ID tag constants
	if SlashAddNoQueryID != "[hop:slash-add-nq]" {
		t.Errorf("SlashAddNoQueryID = %s, want '[hop:slash-add-nq]'", SlashAddNoQueryID)
	}
	if SlashAddWithQueryID != "[hop:slash-add-wq]" {
		t.Errorf("SlashAddWithQueryID = %s, want '[hop:slash-add-wq]'", SlashAddWithQueryID)
	}
}

func TestSlashRuleDescriptions(t *testing.T) {
	// Verify descriptions contain ID tags
	wantNoQuery := "Trailing slash: add (no query) [hop:slash-add-nq]"
	if SlashAddDescNoQuery != wantNoQuery {
		t.Errorf("SlashAddDescNoQuery = %s, want %s", SlashAddDescNoQuery, wantNoQuery)
	}
	wantWithQuery := "Trailing slash: add (with query) [hop:slash-add-wq]"
	if SlashAddDescWithQuery != wantWithQuery {
		t.Errorf("SlashAddDescWithQuery = %s, want %s", SlashAddDescWithQuery, wantWithQuery)
	}
}

func TestTriggerTypeConstants(t *testing.T) {
	// Verify trigger type constants match Bunny API values
	if TriggerTypeURL != 0 {
		t.Errorf("TriggerTypeURL = %d, want 0", TriggerTypeURL)
	}
	if TriggerTypeURLExtension != 3 {
		t.Errorf("TriggerTypeURLExtension = %d, want 3", TriggerTypeURLExtension)
	}
	if TriggerTypeUrlQueryString != 6 {
		t.Errorf("TriggerTypeUrlQueryString = %d, want 6", TriggerTypeUrlQueryString)
	}
}

func TestPatternMatchingConstants(t *testing.T) {
	// Verify pattern matching constants
	if PatternMatchingAny != 0 {
		t.Errorf("PatternMatchingAny = %d, want 0", PatternMatchingAny)
	}
	if PatternMatchingAll != 1 {
		t.Errorf("PatternMatchingAll = %d, want 1", PatternMatchingAll)
	}
	if PatternMatchingNone != 2 {
		t.Errorf("PatternMatchingNone = %d, want 2", PatternMatchingNone)
	}
}

func TestTriggerMatchingConstants(t *testing.T) {
	// Verify trigger matching constants
	if TriggerMatchingAny != 0 {
		t.Errorf("TriggerMatchingAny = %d, want 0", TriggerMatchingAny)
	}
	if TriggerMatchingAll != 1 {
		t.Errorf("TriggerMatchingAll = %d, want 1", TriggerMatchingAll)
	}
}

// redirectRule builds a 302 rule with the given destination and source patterns.
func redirectRule(guid, dest string, patterns ...string) EdgeRuleResponse {
	return EdgeRuleResponse{
		Guid:             guid,
		ActionType:       1,
		ActionParameter1: dest,
		ActionParameter2: "302",
		Enabled:          true,
		Triggers: []Trigger{{
			Type:                TriggerTypeURL,
			PatternMatches:      patterns,
			PatternMatchingType: PatternMatchingAny,
		}},
	}
}

func TestSplitFragment(t *testing.T) {
	tests := []struct {
		name         string
		urlStr       string
		wantBase     string
		wantFragment string
	}{
		{"no fragment", "https://example.com/a/", "https://example.com/a/", ""},
		{"with fragment", "https://example.com/a/#why-me", "https://example.com/a/", "#why-me"},
		{"bare fragment", "#top", "", "#top"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, fragment := splitFragment(tt.urlStr)
			if base != tt.wantBase || fragment != tt.wantFragment {
				t.Errorf("splitFragment(%q) = (%q, %q), want (%q, %q)",
					tt.urlStr, base, fragment, tt.wantBase, tt.wantFragment)
			}
		})
	}
}

func TestExtractSourcePatterns(t *testing.T) {
	rule := EdgeRuleResponse{
		Triggers: []Trigger{
			{Type: TriggerTypeURL, PatternMatches: []string{"/a/", "/b/"}, PatternMatchingType: PatternMatchingAny},
			{Type: TriggerTypeURL, PatternMatches: []string{"/never/"}, PatternMatchingType: PatternMatchingNone},
			{Type: TriggerTypeURLExtension, PatternMatches: []string{".png"}},
		},
	}

	got := extractSourcePatterns(rule)
	want := []string{"/a/", "/b/"}
	if len(got) != len(want) {
		t.Fatalf("extractSourcePatterns() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pattern %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFindRedirectChains(t *testing.T) {
	t.Run("flattens a two hop chain", func(t *testing.T) {
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/cto-coach/", "https://www.example.com/about/"),
			redirectRule("b", "https://www.example.com/coaching/#why-me", "https://www.example.com/cto-coach/"),
		}

		chains, problems := findRedirectChains(rules)
		if len(problems) != 0 {
			t.Fatalf("unexpected problems: %v", problems)
		}
		if len(chains) != 1 {
			t.Fatalf("got %d chains, want 1", len(chains))
		}
		if chains[0].Rule.Guid != "a" {
			t.Errorf("chained rule = %q, want %q", chains[0].Rule.Guid, "a")
		}
		if chains[0].Final != "https://www.example.com/coaching/#why-me" {
			t.Errorf("Final = %q, want the terminal URL", chains[0].Final)
		}
	})

	t.Run("matches a path pattern against a full destination URL", func(t *testing.T) {
		// The /de/ rule in production is written as a path, but the rule it
		// chains into is written as a full URL.
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/de/old/", "/de/"),
			redirectRule("b", "https://www.example.com/de/new/", "https://www.example.com/de/old/"),
		}

		chains, _ := findRedirectChains(rules)
		if len(chains) != 1 {
			t.Fatalf("got %d chains, want 1", len(chains))
		}
		if chains[0].Final != "https://www.example.com/de/new/" {
			t.Errorf("Final = %q, want https://www.example.com/de/new/", chains[0].Final)
		}
	})

	t.Run("leaves direct redirects alone", func(t *testing.T) {
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/new/", "/old/"),
			redirectRule("b", "https://www.example.com/other/", "/different/"),
		}

		chains, problems := findRedirectChains(rules)
		if len(chains) != 0 || len(problems) != 0 {
			t.Errorf("got %d chains and %d problems, want none", len(chains), len(problems))
		}
	})

	t.Run("reports a loop instead of spinning", func(t *testing.T) {
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/b/", "https://www.example.com/a/"),
			redirectRule("b", "https://www.example.com/a/", "https://www.example.com/b/"),
		}

		chains, problems := findRedirectChains(rules)
		if len(problems) == 0 {
			t.Errorf("expected a loop to be reported, got chains %v", chains)
		}
	})

	t.Run("skips rules using Bunny edge variables", func(t *testing.T) {
		// The trailing-slash rules resolve their destination at request time.
		rules := []EdgeRuleResponse{
			redirectRule("slash", "https://www.example.com%{Url.Directory}%{Url.FileName}/", ""),
			redirectRule("a", "https://www.example.com/b/", "/a/"),
		}

		chains, problems := findRedirectChains(rules)
		if len(chains) != 0 || len(problems) != 0 {
			t.Errorf("got %d chains and %d problems, want none", len(chains), len(problems))
		}
	})

	t.Run("carries a fragment picked up mid chain", func(t *testing.T) {
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/b/#section", "/a/"),
			redirectRule("b", "https://www.example.com/c/", "https://www.example.com/b/"),
		}

		chains, _ := findRedirectChains(rules)
		if len(chains) != 1 {
			t.Fatalf("got %d chains, want 1", len(chains))
		}
		if chains[0].Final != "https://www.example.com/c/#section" {
			t.Errorf("Final = %q, want the fragment carried to /c/", chains[0].Final)
		}
	})

	t.Run("does not treat a rule as a hop through itself", func(t *testing.T) {
		// A rule whose destination matches one of its own patterns after
		// normalization must not be reported as a chain.
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/a/", "https://www.example.com/a"),
		}

		chains, problems := findRedirectChains(rules)
		if len(chains) != 0 || len(problems) != 0 {
			t.Errorf("got %d chains and %d problems, want none", len(chains), len(problems))
		}
	})

	t.Run("stops at a disabled rule", func(t *testing.T) {
		disabled := redirectRule("b", "https://www.example.com/c/", "https://www.example.com/b/")
		disabled.Enabled = false
		rules := []EdgeRuleResponse{
			redirectRule("a", "https://www.example.com/b/", "/a/"),
			disabled,
		}

		chains, problems := findRedirectChains(rules)
		if len(chains) != 0 || len(problems) != 0 {
			t.Errorf("got %d chains and %d problems, want none", len(chains), len(problems))
		}
	})
}
