package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Trigger types for edge rules
const (
	TriggerTypeURL            = 0
	TriggerTypeRequestHeader  = 1
	TriggerTypeURLExtension   = 3
	TriggerTypeUrlQueryString = 6
)

// Trigger matching types
const (
	TriggerMatchingAny = 0 // Match ANY trigger
	TriggerMatchingAll = 1 // Match ALL triggers
)

// Pattern matching types
const (
	PatternMatchingAny  = 0 // Match if ANY pattern matches
	PatternMatchingAll  = 1 // Match if ALL patterns match
	PatternMatchingNone = 2 // Match if NONE of the patterns match (negation)
)

type EdgeRule struct {
	Guid                string    `json:"Guid,omitempty"`
	ActionType          int       `json:"ActionType"`
	ActionParameter1    string    `json:"ActionParameter1,omitempty"`
	ActionParameter2    string    `json:"ActionParameter2,omitempty"`
	Triggers            []Trigger `json:"Triggers"`
	TriggerMatchingType int       `json:"TriggerMatchingType"`
	Description         string    `json:"Description,omitempty"`
	Enabled             bool      `json:"Enabled"`
}

type Trigger struct {
	Type                int      `json:"Type"`
	PatternMatches      []string `json:"PatternMatches"`
	PatternMatchingType int      `json:"PatternMatchingType"`
	Parameter1          string   `json:"Parameter1,omitempty"`
}

type EdgeRuleResponse struct {
	Guid                string    `json:"Guid"`
	ActionType          int       `json:"ActionType"`
	ActionParameter1    string    `json:"ActionParameter1"`
	ActionParameter2    string    `json:"ActionParameter2"`
	Triggers            []Trigger `json:"Triggers"`
	TriggerMatchingType int       `json:"TriggerMatchingType"`
	Description         string    `json:"Description"`
	Enabled             bool      `json:"Enabled"`
}

type CheckIssue struct {
	Type     string
	Severity string
	Message  string
	Rule     *EdgeRuleResponse
	Details  map[string]interface{}
}

// CheckResult holds validation results with issues and successful checks
type CheckResult struct {
	Issues     []CheckIssue
	Successful []CheckIssue
}

type RedirectMap struct {
	SourceToDestination map[string]string
	Rules               map[string]*EdgeRuleResponse
}

func addEdgeRule(ctx context.Context, apiKey, zoneID string, rule EdgeRule) error {
	jsonData, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s/edgerules/addOrUpdate", zoneID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	if resp == nil {
		return fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

func listEdgeRules(ctx context.Context, apiKey, zoneID string) ([]EdgeRuleResponse, error) {
	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s", zoneID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var pullZone PullZoneDetails
	if err := json.Unmarshal(body, &pullZone); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	return pullZone.EdgeRules, nil
}

func performHealthCheck(ctx context.Context, targetURL string) (int, bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return 0, false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	if resp == nil {
		return 0, false, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	hasRedirect := resp.StatusCode >= 300 && resp.StatusCode < 400
	return resp.StatusCode, hasRedirect, nil
}

func isValidDomain(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return parsedURL.Host != ""
}

func isSuspiciousURL(urlStr string) (bool, string) {
	suspiciousPatterns := []struct {
		pattern string
		reason  string
	}{
		{`bit\.ly|tinyurl|shortlink|t\.co`, "URL shortener detected"},
		{`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`, "IP address instead of domain"},
		{`[a-z0-9]+-[a-z0-9]+-[a-z0-9]+\.herokuapp\.com`, "Suspicious Heroku subdomain pattern"},
		{`[a-z]{20,}\.com`, "Suspiciously long random domain"},
		{`phishing|malware|scam|fake|fraud`, "Contains suspicious keywords"},
	}

	for _, p := range suspiciousPatterns {
		matched, _ := regexp.MatchString(p.pattern, strings.ToLower(urlStr))
		if matched {
			return true, p.reason
		}
	}
	return false, ""
}

func normalizeURL(urlStr string) string {
	urlStr = strings.ToLower(urlStr)
	if strings.HasSuffix(urlStr, "/") && urlStr != "/" {
		urlStr = strings.TrimSuffix(urlStr, "/")
	}
	return urlStr
}

// extractSourceURL returns the source path a redirect rule matches on.
// Only positive URL triggers qualify: extension/query-string triggers and
// negated URL patterns (e.g. the slash rules' "NOT */") are not sources.
func extractSourceURL(rule EdgeRuleResponse) string {
	for _, trigger := range rule.Triggers {
		if trigger.Type == TriggerTypeURL && trigger.PatternMatchingType != PatternMatchingNone && len(trigger.PatternMatches) > 0 {
			return trigger.PatternMatches[0]
		}
	}
	return ""
}

// hasBunnyVariables reports whether a URL contains Bunny edge variables
// like %{Url.Directory} that only resolve at request time.
func hasBunnyVariables(urlStr string) bool {
	return strings.Contains(urlStr, "%{")
}

func buildRedirectMap(rules []EdgeRuleResponse) *RedirectMap {
	rm := &RedirectMap{
		SourceToDestination: make(map[string]string),
		Rules:               make(map[string]*EdgeRuleResponse),
	}

	for i, rule := range rules {
		if rule.ActionType == 1 && rule.ActionParameter1 != "" {
			source := extractSourceURL(rule)
			if source != "" {
				rm.SourceToDestination[source] = rule.ActionParameter1
				rm.Rules[source] = &rules[i]
			}
		}
	}
	return rm
}

func checkBasicRedirectIssues(rules []EdgeRuleResponse) []CheckIssue {
	var issues []CheckIssue

	for _, rule := range rules {
		if rule.ActionType == 1 { // Redirect action
			// Check for 301 redirects (should be 302)
			if rule.ActionParameter2 == "301" {
				issues = append(issues, CheckIssue{
					Type:     "basic",
					Severity: "warning",
					Message:  "301 redirect detected (should be 302 for temporary redirects)",
					Rule:     &rule,
				})
			}

			// Check for 302 redirects without destination URL
			if rule.ActionParameter2 == "302" && rule.ActionParameter1 == "" {
				issues = append(issues, CheckIssue{
					Type:     "basic",
					Severity: "error",
					Message:  "302 redirect without destination URL",
					Rule:     &rule,
				})
			}

			// Check for rules with destination but no redirect status
			if rule.ActionParameter1 != "" && rule.ActionParameter2 != "302" {
				if rule.ActionParameter2 == "" {
					issues = append(issues, CheckIssue{
						Type:     "basic",
						Severity: "error",
						Message:  "Destination URL set but no redirect status code specified",
						Rule:     &rule,
					})
				} else if rule.ActionParameter2 != "301" {
					issues = append(issues, CheckIssue{
						Type:     "basic",
						Severity: "warning",
						Message:  fmt.Sprintf("Destination URL set but status code is %s (should be 302)", rule.ActionParameter2),
						Rule:     &rule,
					})
				}
			}
		}
	}

	return issues
}

func checkConfigurationIssues(rules []EdgeRuleResponse) []CheckIssue {
	var issues []CheckIssue
	sourceURLs := make(map[string][]*EdgeRuleResponse)

	// Collect all source URLs
	for i, rule := range rules {
		if rule.ActionType == 1 {
			source := extractSourceURL(rule)
			if source != "" {
				sourceURLs[source] = append(sourceURLs[source], &rules[i])

				// Also check normalized version for case/slash issues
				normalized := normalizeURL(source)
				if normalized != source {
					sourceURLs[normalized] = append(sourceURLs[normalized], &rules[i])
				}
			}
		}
	}

	// Check for duplicates and conflicts
	for source, ruleList := range sourceURLs {
		if len(ruleList) > 1 {
			issues = append(issues, CheckIssue{
				Type:     "configuration",
				Severity: "error",
				Message:  fmt.Sprintf("Duplicate/conflicting rules for source path: %s", source),
				Rule:     ruleList[0],
				Details:  map[string]interface{}{"conflict_count": len(ruleList)},
			})
		}
	}

	// Check for case sensitivity and trailing slash issues
	for i, rule := range rules {
		if rule.ActionType == 1 {
			source := extractSourceURL(rule)
			if source != "" {
				// Check for case sensitivity issues
				lowerSource := strings.ToLower(source)
				if lowerSource != source {
					issues = append(issues, CheckIssue{
						Type:     "configuration",
						Severity: "warning",
						Message:  "Mixed case in source URL may cause matching issues",
						Rule:     &rules[i],
					})
				}

				// Check for trailing slash inconsistencies
				if strings.HasSuffix(source, "/") && source != "/" {
					issues = append(issues, CheckIssue{
						Type:     "configuration",
						Severity: "info",
						Message:  "Source URL has trailing slash - ensure this matches expected traffic",
						Rule:     &rules[i],
					})
				}
			}
		}
	}

	return issues
}

func checkSecurityIssues(rules []EdgeRuleResponse, zoneHostnames []Hostname) []CheckIssue {
	var issues []CheckIssue

	for i, rule := range rules {
		if rule.ActionType == 1 && rule.ActionParameter1 != "" {
			destination := rule.ActionParameter1

			// Check for suspicious patterns
			if suspicious, reason := isSuspiciousURL(destination); suspicious {
				issues = append(issues, CheckIssue{
					Type:     "security",
					Severity: "warning",
					Message:  fmt.Sprintf("Suspicious destination URL: %s", reason),
					Rule:     &rules[i],
				})
			}

			// Check for open redirects (external domains)
			destURL, err := url.Parse(destination)
			if err == nil && destURL.Host != "" {
				// This is an absolute URL - check if it's actually external
				isExternal := true
				for _, hostname := range zoneHostnames {
					if strings.EqualFold(destURL.Host, hostname.Value) {
						isExternal = false
						break
					}
				}

				if isExternal {
					issues = append(issues, CheckIssue{
						Type:     "security",
						Severity: "info",
						Message:  "Open redirect to external domain detected",
						Rule:     &rules[i],
						Details:  map[string]interface{}{"external_host": destURL.Host},
					})
				}
			}

			// Check for HTTPS to HTTP downgrades
			if strings.HasPrefix(strings.ToLower(destination), "http://") {
				source := extractSourceURL(rule)
				if strings.Contains(strings.ToLower(source), "https://") {
					issues = append(issues, CheckIssue{
						Type:     "security",
						Severity: "error",
						Message:  "HTTPS to HTTP downgrade detected - security risk",
						Rule:     &rules[i],
					})
				}
			}
		}
	}

	return issues
}

func checkRedirectLoops(redirectMap *RedirectMap) []CheckIssue {
	var issues []CheckIssue

	for source, destination := range redirectMap.SourceToDestination {
		visited := make(map[string]bool)
		current := destination
		chainLength := 0

		// Follow the redirect chain
		for {
			chainLength++
			if chainLength >= 10 {
				issues = append(issues, CheckIssue{
					Type:     "redirect_chain",
					Severity: "error",
					Message:  "Redirect chain too long (>10 hops)",
					Rule:     redirectMap.Rules[source],
				})
				break
			}

			if visited[current] {
				issues = append(issues, CheckIssue{
					Type:     "redirect_loop",
					Severity: "error",
					Message:  "Infinite redirect loop detected",
					Rule:     redirectMap.Rules[source],
					Details:  map[string]interface{}{"loop_url": current},
				})
				break
			}

			visited[current] = true

			// Check if current destination is also a source for another redirect
			nextDest, exists := redirectMap.SourceToDestination[current]
			if !exists {
				if chainLength > 1 {
					issues = append(issues, CheckIssue{
						Type:     "redirect_chain",
						Severity: "warning",
						Message:  fmt.Sprintf("Redirect chain detected (%d hops)", chainLength),
						Rule:     redirectMap.Rules[source],
					})
				}
				break
			}

			current = nextDest
		}
	}

	return issues
}

func checkURLHealth(ctx context.Context, rules []EdgeRuleResponse) []CheckIssue {
	var issues []CheckIssue

	for i, rule := range rules {
		if rule.ActionType == 1 && rule.ActionParameter1 != "" {
			destination := rule.ActionParameter1

			// Skip relative URLs for health checks
			if !strings.HasPrefix(destination, "http") {
				continue
			}

			// Templated destinations can't be validated or fetched statically
			if hasBunnyVariables(destination) {
				continue
			}

			// Validate domain first
			if !isValidDomain(destination) {
				issues = append(issues, CheckIssue{
					Type:     "url_health",
					Severity: "error",
					Message:  "Invalid destination URL format",
					Rule:     &rules[i],
				})
				continue
			}

			// Perform health check
			statusCode, hasRedirect, err := performHealthCheck(ctx, destination)
			if err != nil {
				issues = append(issues, CheckIssue{
					Type:     "url_health",
					Severity: "error",
					Message:  fmt.Sprintf("URL health check failed: %v", err),
					Rule:     &rules[i],
				})
				continue
			}

			// Check for broken URLs
			if statusCode >= 400 {
				severity := "error"
				if statusCode >= 500 {
					severity = "critical"
				}
				issues = append(issues, CheckIssue{
					Type:     "url_health",
					Severity: severity,
					Message:  fmt.Sprintf("Broken destination URL (HTTP %d)", statusCode),
					Rule:     &rules[i],
				})
			}

			// Check for additional redirects
			if hasRedirect {
				issues = append(issues, CheckIssue{
					Type:     "url_health",
					Severity: "info",
					Message:  "Destination URL itself redirects (creating a redirect chain)",
					Rule:     &rules[i],
				})
			}
		}
	}

	return issues
}

// checkRulesStructured performs all rules validation and returns structured results
func checkRulesStructured(ctx context.Context, apiKey, zoneID string, skipHealth bool) (CheckResult, error) {
	var result CheckResult

	// Get all edge rules
	rules, err := listEdgeRules(ctx, apiKey, zoneID)
	if err != nil {
		return result, fmt.Errorf("error listing edge rules: %v", err)
	}

	var allIssues []CheckIssue
	redirectMap := buildRedirectMap(rules)

	// Get pull zone details for hostname information
	pullZoneDetails, err := getPullZoneDetails(ctx, apiKey, zoneID)
	if err != nil {
		pullZoneDetails = &PullZoneDetails{}
	}

	// Run all checks
	allIssues = append(allIssues, checkBasicRedirectIssues(rules)...)
	allIssues = append(allIssues, checkConfigurationIssues(rules)...)
	allIssues = append(allIssues, checkSecurityIssues(rules, pullZoneDetails.Hostnames)...)
	allIssues = append(allIssues, checkRedirectLoops(redirectMap)...)

	if !skipHealth {
		allIssues = append(allIssues, checkURLHealth(ctx, rules)...)
	}

	// Separate issues from info/successful items
	for _, issue := range allIssues {
		if issue.Severity == "critical" || issue.Severity == "error" || issue.Severity == "warning" {
			result.Issues = append(result.Issues, issue)
		} else {
			result.Successful = append(result.Successful, issue)
		}
	}

	return result, nil
}

func displayCheckResults(issues []CheckIssue) {
	if len(issues) == 0 {
		fmt.Printf("No issues found! All redirect rules appear to be properly configured.\n")
		return
	}

	// Group issues by severity
	critical := []CheckIssue{}
	errors := []CheckIssue{}
	warnings := []CheckIssue{}
	info := []CheckIssue{}

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			critical = append(critical, issue)
		case "error":
			errors = append(errors, issue)
		case "warning":
			warnings = append(warnings, issue)
		case "info":
			info = append(info, issue)
		}
	}

	// Display summary
	fmt.Printf("\nANALYSIS SUMMARY:\n")
	fmt.Printf("   Critical: %d\n", len(critical))
	fmt.Printf("   Errors: %d\n", len(errors))
	fmt.Printf("   Warnings: %d\n", len(warnings))
	fmt.Printf("   Info: %d\n", len(info))
	fmt.Println()

	// Display issues by severity
	displayIssueGroup("CRITICAL ISSUES", critical)
	displayIssueGroup("ERRORS", errors)
	displayIssueGroup("WARNINGS", warnings)
	displayIssueGroup("INFORMATION", info)
}

func displayIssueGroup(title string, issues []CheckIssue) {
	if len(issues) == 0 {
		return
	}

	fmt.Printf("%s (%d)\n", title, len(issues))
	fmt.Println(strings.Repeat("─", 50))

	for i, issue := range issues {
		fmt.Printf("\n[%d] %s\n", i+1, issue.Message)
		if issue.Rule != nil {
			fmt.Printf("    Rule: %s\n", issue.Rule.Description)
			fmt.Printf("    GUID: %s\n", issue.Rule.Guid)
			fmt.Printf("    Status: %s\n", map[bool]string{true: "Enabled", false: "Disabled"}[issue.Rule.Enabled])

			// Display all source patterns
			if len(issue.Rule.Triggers) > 0 && len(issue.Rule.Triggers[0].PatternMatches) > 0 {
				patterns := issue.Rule.Triggers[0].PatternMatches
				if len(patterns) == 1 {
					fmt.Printf("    From: %s\n", patterns[0])
				} else {
					fmt.Printf("    From: (%d patterns)\n", len(patterns))
					for _, pattern := range patterns {
						fmt.Printf("      - %s\n", pattern)
					}
				}
			}
			if issue.Rule.ActionParameter1 != "" {
				fmt.Printf("    To: %s\n", issue.Rule.ActionParameter1)
			}
			if issue.Rule.ActionParameter2 != "" {
				fmt.Printf("    Status Code: %s\n", issue.Rule.ActionParameter2)
			}
		}

		// Display additional details
		if issue.Details != nil {
			for key, value := range issue.Details {
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
	}
	fmt.Println()
}

// Rule ID tags for identification (survives description edits)
const (
	SlashAddNoQueryID   = "[hop:slash-add-nq]"
	SlashAddWithQueryID = "[hop:slash-add-wq]"
)

// Slash redirect rule descriptions with ID tags
const (
	SlashAddDescNoQuery   = "Trailing slash: add (no query) " + SlashAddNoQueryID
	SlashAddDescWithQuery = "Trailing slash: add (with query) " + SlashAddWithQueryID
)

// createSlashAddRules creates edge rules for adding trailing slashes to extensionless URLs
func createSlashAddRules(ctx context.Context, apiKey, zoneID, host string) error {
	// Rule 1: Without query string
	ruleNoQuery := EdgeRule{
		ActionType:          ActionTypeRedirect,
		ActionParameter1:    fmt.Sprintf("https://%s%%{Url.Directory}%%{Url.FileName}/", host),
		ActionParameter2:    "302",
		TriggerMatchingType: TriggerMatchingAll,
		Description:         SlashAddDescNoQuery,
		Enabled:             true,
		Triggers: []Trigger{
			{
				Type:                TriggerTypeURLExtension,
				PatternMatches:      []string{"{{empty}}"},
				PatternMatchingType: PatternMatchingAny,
			},
			{
				Type:                TriggerTypeURL,
				PatternMatches:      []string{"*/"},
				PatternMatchingType: PatternMatchingNone,
			},
			{
				Type:                TriggerTypeUrlQueryString,
				PatternMatches:      []string{"?*=*"},
				PatternMatchingType: PatternMatchingNone,
			},
		},
	}

	fmt.Printf("Creating rule: %s\n", SlashAddDescNoQuery)
	if err := addEdgeRule(ctx, apiKey, zoneID, ruleNoQuery); err != nil {
		return fmt.Errorf("failed to create slash-add rule (no query): %w", err)
	}

	// Rule 2: With query string
	ruleWithQuery := EdgeRule{
		ActionType:          ActionTypeRedirect,
		ActionParameter1:    fmt.Sprintf("https://%s%%{Url.Directory}%%{Url.FileName}/?%%{Request.QueryString}", host),
		ActionParameter2:    "302",
		TriggerMatchingType: TriggerMatchingAll,
		Description:         SlashAddDescWithQuery,
		Enabled:             true,
		Triggers: []Trigger{
			{
				Type:                TriggerTypeURLExtension,
				PatternMatches:      []string{"{{empty}}"},
				PatternMatchingType: PatternMatchingAny,
			},
			{
				Type:                TriggerTypeURL,
				PatternMatches:      []string{"*/"},
				PatternMatchingType: PatternMatchingNone,
			},
			{
				Type:                TriggerTypeUrlQueryString,
				PatternMatches:      []string{"?*=*"},
				PatternMatchingType: PatternMatchingAny,
			},
		},
	}

	fmt.Printf("Creating rule: %s\n", SlashAddDescWithQuery)
	if err := addEdgeRule(ctx, apiKey, zoneID, ruleWithQuery); err != nil {
		return fmt.Errorf("failed to create slash-add rule (with query): %w", err)
	}

	return nil
}

// deleteSlashRules removes existing slash redirect rules by ID tag (uses substring matching)
func deleteSlashRules(ctx context.Context, apiKey, zoneID string, idTags []string) (int, error) {
	rules, err := listEdgeRules(ctx, apiKey, zoneID)
	if err != nil {
		return 0, fmt.Errorf("error listing edge rules: %w", err)
	}

	deleted := 0
	for _, rule := range rules {
		for _, tag := range idTags {
			if strings.Contains(rule.Description, tag) {
				fmt.Printf("Removing rule: %s\n", rule.Description)
				if err := deleteEdgeRule(ctx, apiKey, zoneID, rule.Guid); err != nil {
					return deleted, fmt.Errorf("failed to delete rule %s: %w", rule.Guid, err)
				}
				deleted++
				break
			}
		}
	}

	return deleted, nil
}

// deleteEdgeRule removes an edge rule by GUID
func deleteEdgeRule(ctx context.Context, apiKey, zoneID, guid string) error {
	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s/edgerules/%s", zoneID, guid)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

// RedirectChain describes a redirect rule whose destination is itself
// redirected, so a visitor takes more than one hop to reach the final page.
type RedirectChain struct {
	Rule    *EdgeRuleResponse
	Sources []string
	Current string   // the destination the rule points at today
	Final   string   // the terminal destination the chain resolves to
	Hops    []string // intermediate destinations, in order, ending at Final
}

// extractSourcePatterns returns every source pattern a redirect rule matches
// on. extractSourceURL only returns the first, which is enough for reporting
// but not for resolving chains: a rule that redirects four patterns is the
// destination of four different URLs.
func extractSourcePatterns(rule EdgeRuleResponse) []string {
	var patterns []string
	for _, trigger := range rule.Triggers {
		if trigger.Type == TriggerTypeURL && trigger.PatternMatchingType != PatternMatchingNone {
			patterns = append(patterns, trigger.PatternMatches...)
		}
	}
	return patterns
}

// splitFragment separates a URL from its #fragment.
func splitFragment(urlStr string) (base, fragment string) {
	if i := strings.Index(urlStr, "#"); i >= 0 {
		return urlStr[:i], urlStr[i:]
	}
	return urlStr, ""
}

// urlPath returns the path portion of a URL, or the input unchanged when it is
// already a bare path. Bunny patterns are written both ways.
func urlPath(urlStr string) string {
	if u, err := url.Parse(urlStr); err == nil && u.Host != "" {
		return u.Path
	}
	return urlStr
}

// buildSourceIndex maps normalized source patterns to their rules, so a
// destination URL can be looked up as the source of a further redirect. Both
// the full URL and the bare path are indexed because Bunny patterns use either
// form. Path keys are only unambiguous within a single pull zone, which is the
// scope hop operates on.
func buildSourceIndex(rules []EdgeRuleResponse) map[string]*EdgeRuleResponse {
	index := make(map[string]*EdgeRuleResponse)
	for i, rule := range rules {
		if rule.ActionType != 1 || hasBunnyVariables(rule.ActionParameter1) {
			continue
		}
		for _, pattern := range extractSourcePatterns(rule) {
			if hasBunnyVariables(pattern) {
				continue
			}
			base, _ := splitFragment(pattern)
			for _, key := range []string{normalizeURL(base), normalizeURL(urlPath(base))} {
				if key == "" {
					continue
				}
				if _, exists := index[key]; !exists {
					index[key] = &rules[i]
				}
			}
		}
	}
	return index
}

// maxRedirectDepth bounds chain resolution so a redirect loop cannot spin.
const maxRedirectDepth = 10

// resolveFinalDestination follows dest through any further redirect rules and
// returns the terminal URL plus the hops taken to reach it. A fragment picked
// up along the way is carried to the final URL when the final URL has none, so
// flattening /a -> /b#section -> /c lands on /c#section rather than /c.
// Returns an error on a loop or on exceeding maxRedirectDepth.
func resolveFinalDestination(dest string, index map[string]*EdgeRuleResponse, self *EdgeRuleResponse) (string, []string, error) {
	var hops []string
	seen := map[string]bool{}
	carriedFragment := ""
	current := dest

	for depth := 0; depth < maxRedirectDepth; depth++ {
		base, fragment := splitFragment(current)
		if fragment != "" {
			carriedFragment = fragment
		}

		key := normalizeURL(base)
		next, ok := index[key]
		if !ok {
			next, ok = index[normalizeURL(urlPath(base))]
		}
		// A terminal URL has no rule. A rule whose own destination normalizes
		// onto one of its own patterns (say /a -> /a/) is terminal too, but
		// only at the first hop: coming back to the starting rule later means
		// the chain loops, which the seen check below reports.
		isSelfAtStart := depth == 0 && self != nil && ok && next != nil && next.Guid == self.Guid
		if !ok || next == nil || isSelfAtStart {
			if carriedFragment != "" && !strings.Contains(current, "#") {
				current += carriedFragment
			}
			return current, hops, nil
		}
		if !next.Enabled {
			return current, hops, nil
		}
		if seen[key] {
			return "", hops, fmt.Errorf("redirect loop at %s", current)
		}
		seen[key] = true

		current = next.ActionParameter1
		if current == "" || hasBunnyVariables(current) {
			return "", hops, fmt.Errorf("cannot resolve destination of rule %s", next.Guid)
		}
		hops = append(hops, current)
	}
	return "", hops, fmt.Errorf("redirect chain longer than %d hops starting at %s", maxRedirectDepth, dest)
}

// findRedirectChains returns every 302 rule that points at a URL which is
// itself redirected, along with the terminal destination each one should point
// at instead. Rules using Bunny edge variables are skipped: their destination
// is only known at request time.
func findRedirectChains(rules []EdgeRuleResponse) ([]RedirectChain, []error) {
	index := buildSourceIndex(rules)

	var chains []RedirectChain
	var problems []error

	for i, rule := range rules {
		if rule.ActionType != 1 || rule.ActionParameter2 != "302" {
			continue
		}
		if rule.ActionParameter1 == "" || hasBunnyVariables(rule.ActionParameter1) {
			continue
		}

		final, hops, err := resolveFinalDestination(rule.ActionParameter1, index, &rules[i])
		if err != nil {
			problems = append(problems, fmt.Errorf("rule %s (%s): %w", rule.Guid, rule.Description, err))
			continue
		}
		if len(hops) == 0 || final == rule.ActionParameter1 {
			continue
		}
		chains = append(chains, RedirectChain{
			Rule:    &rules[i],
			Sources: extractSourcePatterns(rule),
			Current: rule.ActionParameter1,
			Final:   final,
			Hops:    hops,
		})
	}
	return chains, problems
}

// retargetEdgeRule rewrites an existing rule's destination in place, keeping
// its GUID, patterns and description.
func retargetEdgeRule(ctx context.Context, apiKey, zoneID string, rule *EdgeRuleResponse, newDest string) error {
	updated := EdgeRule{
		Guid:                rule.Guid,
		ActionType:          rule.ActionType,
		ActionParameter1:    newDest,
		ActionParameter2:    rule.ActionParameter2,
		TriggerMatchingType: rule.TriggerMatchingType,
		Description:         rule.Description,
		Enabled:             rule.Enabled,
		Triggers:            rule.Triggers,
	}
	return addEdgeRule(ctx, apiKey, zoneID, updated)
}
