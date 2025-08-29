package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kong"
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

type PullZone struct {
	Id   int64  `json:"Id"`
	Name string `json:"Name"`
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

type PullZoneDetails struct {
	Id        int64              `json:"Id"`
	Name      string             `json:"Name"`
	EdgeRules []EdgeRuleResponse `json:"EdgeRules"`
	Hostnames []Hostname         `json:"Hostnames"`
}

type Hostname struct {
	Id    int64  `json:"Id"`
	Value string `json:"Value"`
}

type CheckIssue struct {
	Type     string
	Severity string
	Message  string
	Rule     *EdgeRuleResponse
	Details  map[string]interface{}
}

type RedirectMap struct {
	SourceToDestination map[string]string
	Rules               map[string]*EdgeRuleResponse
}

var CLI struct {
	Rules struct {
		Add struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
			From string `kong:"required,help='Source URL path to redirect from'"`
			To   string `kong:"required,help='Destination URL to redirect to'"`
			Desc string `kong:"help='Edge rule description'"`
		} `kong:"cmd,help='Add a new 302 redirect'"`

		List struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
		} `kong:"cmd,help='List all existing 302 redirects'"`

		Check struct {
			Key        string `kong:"required,help='Bunny CDN API key'"`
			Zone       string `kong:"required,help='Pull Zone name'"`
			SkipHealth bool   `kong:"help='Skip HTTP health checks for faster execution'"`
		} `kong:"cmd,help='Check redirect rules for potential issues'"`
	} `kong:"cmd,help='Manage redirect rules'"`
}

func findPullZoneByName(apiKey, name string) (int64, error) {
	req, err := http.NewRequest("GET", "https://api.bunny.net/pullzone", nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error making request: %v", err)
	}
	if resp == nil {
		return 0, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response: %v", err)
	}

	var pullZones []PullZone
	if err := json.Unmarshal(body, &pullZones); err != nil {
		return 0, fmt.Errorf("error parsing JSON response: %v", err)
	}

	// Search for the pull zone by name
	for _, zone := range pullZones {
		if strings.EqualFold(zone.Name, name) {
			return zone.Id, nil
		}
	}

	return 0, fmt.Errorf("pull zone with name '%s' not found", name)
}

func addEdgeRule(apiKey, zoneID string, rule EdgeRule) error {
	jsonData, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s/edgerules/addOrUpdate", zoneID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
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

func getPullZoneDetails(apiKey, zoneID string) (*PullZoneDetails, error) {
	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s", zoneID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{}
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

	return &pullZone, nil
}

func listEdgeRules(apiKey, zoneID string) ([]EdgeRuleResponse, error) {
	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s", zoneID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{}
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

func performHealthCheck(targetURL string) (int, bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return 0, false, err
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

func extractSourceURL(rule EdgeRuleResponse) string {
	if len(rule.Triggers) > 0 && len(rule.Triggers[0].PatternMatches) > 0 {
		return rule.Triggers[0].PatternMatches[0]
	}
	return ""
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

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("hop"),
		kong.Description("A Go command-line tool to manage 302 redirects in Bunny CDN pull zones."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}))

	switch ctx.Command() {
	case "rules add":
		handleAdd()
	case "rules list":
		handleList()
	case "rules check":
		handleCheck()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", ctx.Command())
		_ = ctx.PrintUsage(true)
		os.Exit(1)
	}
}


func handleAdd() {
	// Look up pull zone by name
	id, err := findPullZoneByName(CLI.Rules.Add.Key, CLI.Rules.Add.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Rules.Add.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Rules.Add.Zone, zoneID)

	// Set default description if not provided
	desc := CLI.Rules.Add.Desc
	if desc == "" {
		desc = fmt.Sprintf("302 redirect from %s to %s", CLI.Rules.Add.From, CLI.Rules.Add.To)
	}

	// Create the edge rule for 302 redirect using the Redirect action
	rule := EdgeRule{
		ActionType:          1,                        // Redirect
		ActionParameter1:    CLI.Rules.Add.To,         // Destination URL
		ActionParameter2:    "302",                    // Status code
		TriggerMatchingType: 0,                        // MatchAny
		Description:         desc,
		Enabled:             true,
		Triggers: []Trigger{
			{
				Type:                0,                                // Url trigger
				PatternMatches:      []string{CLI.Rules.Add.From},
				PatternMatchingType: 0,                               // MatchAny
			},
		},
	}

	err = addEdgeRule(CLI.Rules.Add.Key, zoneID, rule)
	if err != nil {
		log.Fatalf("Error adding edge rule: %v", err)
	}

	fmt.Printf("Successfully added 302 redirect from %s to %s\n", CLI.Rules.Add.From, CLI.Rules.Add.To)
}

func handleList() {
	// Look up pull zone by name
	id, err := findPullZoneByName(CLI.Rules.List.Key, CLI.Rules.List.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Rules.List.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Rules.List.Zone, zoneID)

	// Get all edge rules
	rules, err := listEdgeRules(CLI.Rules.List.Key, zoneID)
	if err != nil {
		log.Fatalf("Error listing edge rules: %v", err)
	}

	// Filter and display 302 redirects
	redirects := []EdgeRuleResponse{}
	for _, rule := range rules {
		if rule.ActionType == 1 && rule.ActionParameter2 == "302" {
			redirects = append(redirects, rule)
		}
	}

	if len(redirects) == 0 {
		fmt.Println("No 302 redirects found in this pull zone.")
		return
	}

	fmt.Printf("\nFound %d 302 redirect(s):\n", len(redirects))
	fmt.Println("=" + strings.Repeat("=", 70))

	for i, redirect := range redirects {
		fmt.Printf("\n%d. %s\n", i+1, redirect.Description)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "Enabled", false: "Disabled"}[redirect.Enabled])

		// Extract source URL from triggers
		if len(redirect.Triggers) > 0 && len(redirect.Triggers[0].PatternMatches) > 0 {
			fmt.Printf("   From: %s\n", redirect.Triggers[0].PatternMatches[0])
		}

		fmt.Printf("   To: %s\n", redirect.ActionParameter1)
		fmt.Printf("   GUID: %s\n", redirect.Guid)
	}
}

func handleCheck() {
	// Look up pull zone by name
	id, err := findPullZoneByName(CLI.Rules.Check.Key, CLI.Rules.Check.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Rules.Check.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Rules.Check.Zone, zoneID)

	// Get all edge rules
	rules, err := listEdgeRules(CLI.Rules.Check.Key, zoneID)
	if err != nil {
		log.Fatalf("Error listing edge rules: %v", err)
	}

	fmt.Printf("\nRunning comprehensive redirect analysis on %d edge rules...\n", len(rules))
	fmt.Println("=" + strings.Repeat("=", 70))

	var allIssues []CheckIssue
	redirectMap := buildRedirectMap(rules)

	// Get pull zone details for hostname information
	pullZoneDetails, err := getPullZoneDetails(CLI.Rules.Check.Key, zoneID)
	if err != nil {
		log.Printf("Warning: Could not get pull zone details for hostname checking: %v", err)
		pullZoneDetails = &PullZoneDetails{}
	}

	// Run all checks
	allIssues = append(allIssues, checkBasicRedirectIssues(rules)...)
	allIssues = append(allIssues, checkConfigurationIssues(rules)...)
	allIssues = append(allIssues, checkSecurityIssues(rules, pullZoneDetails.Hostnames)...)
	allIssues = append(allIssues, checkRedirectLoops(redirectMap)...)

	if !CLI.Rules.Check.SkipHealth {
		fmt.Println("Running HTTP health checks... (use --skip-health to skip)")
		allIssues = append(allIssues, checkURLHealth(rules)...)
	}

	// Display results
	displayCheckResults(allIssues)
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
			if chainLength > 10 {
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

func checkURLHealth(rules []EdgeRuleResponse) []CheckIssue {
	var issues []CheckIssue

	for i, rule := range rules {
		if rule.ActionType == 1 && rule.ActionParameter1 != "" {
			destination := rule.ActionParameter1

			// Skip relative URLs for health checks
			if !strings.HasPrefix(destination, "http") {
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
			statusCode, hasRedirect, err := performHealthCheck(destination)
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

func displayCheckResults(issues []CheckIssue) {
	if len(issues) == 0 {
		fmt.Printf("\n✅ No issues found! All redirect rules appear to be properly configured.\n")
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
	fmt.Printf("\n📊 ANALYSIS SUMMARY:\n")
	fmt.Printf("   🔴 Critical: %d\n", len(critical))
	fmt.Printf("   ❌ Errors: %d\n", len(errors))
	fmt.Printf("   ⚠️  Warnings: %d\n", len(warnings))
	fmt.Printf("   ℹ️  Info: %d\n", len(info))
	fmt.Println()

	// Display issues by severity
	displayIssueGroup("🔴 CRITICAL ISSUES", critical)
	displayIssueGroup("❌ ERRORS", errors)
	displayIssueGroup("⚠️  WARNINGS", warnings)
	displayIssueGroup("ℹ️  INFORMATION", info)
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

			source := extractSourceURL(*issue.Rule)
			if source != "" {
				fmt.Printf("    From: %s\n", source)
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
