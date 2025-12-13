package main

import (
	"context"
	"fmt"
	"strings"
)

// Edge rule action types
const (
	ActionTypeForceSSL          = 0
	ActionTypeRedirect          = 1
	ActionTypeOriginURL         = 2
	ActionTypeOverrideCacheTime = 3
	ActionTypeBlockRequest      = 4
	ActionTypeSetResponseHeader = 5
	ActionTypeSetRequestHeader  = 6
)

// SecurityHeader represents a recommended security HTTP header
type SecurityHeader struct {
	Name           string
	RecommendedVal string
	Description    string
	Severity       string // "error" = fail check, "warning" = just report
}

// recommendedHeaders lists security headers to check
// Based on OWASP HTTP Headers Cheat Sheet
// Excludes dangerous headers like HPKP (can brick sites) and CSP (too site-specific)
var recommendedHeaders = []SecurityHeader{
	// Critical headers (ERROR if missing)
	{Name: "Strict-Transport-Security", RecommendedVal: "max-age=63072000; includeSubDomains", Description: "Forces HTTPS connections", Severity: "error"},
	{Name: "X-Frame-Options", RecommendedVal: "DENY", Description: "Prevents clickjacking", Severity: "error"},
	{Name: "X-Content-Type-Options", RecommendedVal: "nosniff", Description: "Prevents MIME sniffing", Severity: "error"},
	// Recommended headers (WARNING if missing)
	{Name: "Referrer-Policy", RecommendedVal: "strict-origin-when-cross-origin", Description: "Controls referrer information", Severity: "warning"},
	{Name: "X-XSS-Protection", RecommendedVal: "0", Description: "Disables legacy XSS filter", Severity: "warning"},
	{Name: "Permissions-Policy", RecommendedVal: "geolocation=(), camera=(), microphone=()", Description: "Restricts browser features", Severity: "warning"},
	{Name: "Cross-Origin-Opener-Policy", RecommendedVal: "same-origin", Description: "Isolates browsing context", Severity: "warning"},
	{Name: "Cross-Origin-Embedder-Policy", RecommendedVal: "require-corp", Description: "Blocks cross-origin resources", Severity: "warning"},
	{Name: "Cross-Origin-Resource-Policy", RecommendedVal: "same-site", Description: "Limits resource loading", Severity: "warning"},
}

// checkSecurityHeaders validates that recommended security headers are configured as edge rules
func checkSecurityHeaders(ctx context.Context, apiKey, zoneID string) (CheckResult, error) {
	// Fetch all edge rules
	rules, err := listEdgeRules(ctx, apiKey, zoneID)
	if err != nil {
		return CheckResult{}, fmt.Errorf("error listing edge rules: %v", err)
	}

	return analyzeSecurityHeaders(rules), nil
}

// analyzeSecurityHeaders checks edge rules against recommended security headers
// This is separated from checkSecurityHeaders for testability
func analyzeSecurityHeaders(rules []EdgeRuleResponse) CheckResult {
	var result CheckResult

	// Build a map of configured response headers (header name -> value)
	configuredHeaders := make(map[string]string)
	for _, rule := range rules {
		if rule.ActionType == ActionTypeSetResponseHeader && rule.Enabled {
			// ActionParameter1 = header name, ActionParameter2 = header value
			headerName := strings.ToLower(rule.ActionParameter1)
			configuredHeaders[headerName] = rule.ActionParameter2
		}
	}

	// Check each recommended header
	for _, header := range recommendedHeaders {
		headerNameLower := strings.ToLower(header.Name)
		configuredValue, exists := configuredHeaders[headerNameLower]

		if !exists {
			// Header not configured
			severity := header.Severity
			prefix := "WARN"
			if severity == "error" {
				prefix = "ERROR"
			}
			result.Issues = append(result.Issues, CheckIssue{
				Type:     "security_header_missing",
				Severity: severity,
				Message:  fmt.Sprintf("%s: %s - Not configured (recommended: %s)", header.Name, prefix, header.RecommendedVal),
				Details: map[string]interface{}{
					"header":      header.Name,
					"recommended": header.RecommendedVal,
					"description": header.Description,
				},
			})
		} else {
			// Header is configured - report as OK
			result.Successful = append(result.Successful, CheckIssue{
				Type:     "security_header_ok",
				Severity: "info",
				Message:  fmt.Sprintf("%s: OK (%s)", header.Name, configuredValue),
				Details: map[string]interface{}{
					"header": header.Name,
					"value":  configuredValue,
				},
			})
		}
	}

	return result
}

// fixSecurityHeaders adds missing security headers as edge rules
func fixSecurityHeaders(ctx context.Context, apiKey, zoneID string) error {
	// Fetch all edge rules
	rules, err := listEdgeRules(ctx, apiKey, zoneID)
	if err != nil {
		return fmt.Errorf("error listing edge rules: %v", err)
	}

	// Build a map of configured response headers (header name -> value)
	configuredHeaders := make(map[string]string)
	for _, rule := range rules {
		if rule.ActionType == ActionTypeSetResponseHeader && rule.Enabled {
			headerName := strings.ToLower(rule.ActionParameter1)
			configuredHeaders[headerName] = rule.ActionParameter2
		}
	}

	fmt.Println("\nADDING MISSING SECURITY HEADERS")
	fmt.Println(strings.Repeat("-", 40))

	added := 0
	skipped := 0

	// Check each recommended header and add if missing
	for _, header := range recommendedHeaders {
		headerNameLower := strings.ToLower(header.Name)
		if _, exists := configuredHeaders[headerNameLower]; exists {
			fmt.Printf("%s: Already configured\n", header.Name)
			skipped++
			continue
		}

		// Create edge rule for this header
		// Note: Bunny CDN requires at least one trigger - use "*" to match all URLs
		rule := EdgeRule{
			ActionType:       ActionTypeSetResponseHeader,
			ActionParameter1: header.Name,
			ActionParameter2: header.RecommendedVal,
			Triggers: []Trigger{{
				Type:                0,            // URL trigger
				PatternMatches:      []string{"*"}, // Match all URLs
				PatternMatchingType: 0,
			}},
			TriggerMatchingType: 0,
			Description:         fmt.Sprintf("Security header: %s", header.Name),
			Enabled:             true,
		}

		err := addEdgeRule(ctx, apiKey, zoneID, rule)
		if err != nil {
			fmt.Printf("%s: FAILED - %v\n", header.Name, err)
			continue
		}

		fmt.Printf("%s: Added (%s)\n", header.Name, header.RecommendedVal)
		added++
	}

	// Summary
	fmt.Printf("\nSUMMARY: %d headers added, %d already configured\n", added, skipped)

	return nil
}

// displaySecurityResults shows the security check results
func displaySecurityResults(result CheckResult) {
	fmt.Println("\nSECURITY HEADERS CHECK")
	fmt.Println(strings.Repeat("-", 40))

	// Display successful checks first
	for _, success := range result.Successful {
		fmt.Println(success.Message)
	}

	// Display issues
	for _, issue := range result.Issues {
		fmt.Println(issue.Message)
	}

	// Summary
	configured := len(result.Successful)
	missing := len(result.Issues)

	headerWord := "header"
	if configured != 1 {
		headerWord = "headers"
	}
	missingWord := "header"
	if missing != 1 {
		missingWord = "headers"
	}

	fmt.Printf("\nSUMMARY: %d %s configured, %d %s missing\n", configured, headerWord, missing, missingWord)
}
