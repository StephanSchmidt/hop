package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

// createDebugContext creates a context with debug flag from global CLI
func createDebugContext(baseCtx context.Context) context.Context {
	return context.WithValue(baseCtx, struct{ key string }{"debug"}, CLI.Debug)
}

var CLI struct {
	Debug bool `kong:"help='Enable debug output'"`

	Check struct {
		Key        string `kong:"required,help='Bunny CDN API key'"`
		Zone       string `kong:"required,help='Pull Zone name'"`
		SkipHealth bool   `kong:"help='Skip HTTP health checks for faster execution'"`
	} `kong:"cmd,help='Run all checks (rules, DNS, SSL) for a pull zone'"`

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

	CDN struct {
		Push struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
			From string `kong:"required,help='Local directory path to upload from'"`
		} `kong:"cmd,help='Push files from local directory to CDN storage'"`

		Check struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
		} `kong:"cmd,help='Check SSL configuration for all pull zone hostnames'"`
	} `kong:"cmd,help='Manage CDN content'"`

	DNS struct {
		List struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
		} `kong:"cmd,help='List DNS A and CNAME records for a pull zone'"`

		Check struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
		} `kong:"cmd,help='Check DNS records exist for pull zone hostnames'"`
	} `kong:"cmd,help='Manage DNS records'"`
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
	case "check":
		handleGeneralCheck()
	case "rules add":
		handleAdd()
	case "rules list":
		handleList()
	case "rules check":
		handleCheck()
	case "cdn push":
		handleCDNPush()
	case "cdn check":
		handleCDNCheck()
	case "dns list":
		handleDNSList()
	case "dns check":
		handleDNSCheck()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", ctx.Command())
		_ = ctx.PrintUsage(true)
		os.Exit(1)
	}
}

func handleCDNPush() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Verify local directory exists
	localDir := CLI.CDN.Push.From
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		log.Fatalf("Local directory '%s' does not exist", localDir)
	}

	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, CLI.CDN.Push.Key, CLI.CDN.Push.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.CDN.Push.Zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", CLI.CDN.Push.Zone, pullZoneID)

	// Find associated storage zone
	storageZone, err := getStorageZoneByPullZone(ctx, CLI.CDN.Push.Key, pullZoneID)
	if err != nil {
		log.Fatalf("Error finding storage zone: %v", err)
	}
	fmt.Printf("Found storage zone: %s\n", storageZone.Name)

	// Upload directory contents
	fmt.Printf("Uploading files from '%s' to storage zone '%s'...\n", localDir, storageZone.Name)

	results := uploadDirectoryOptimized(ctx, storageZone, localDir, "")

	// Summary
	successful := 0
	skipped := 0
	failed := 0
	for _, result := range results {
		if result.Success {
			if result.Skipped {
				skipped++
			} else {
				successful++
			}
		} else {
			failed++
		}
	}

	uploadedWord := "file"
	if successful != 1 {
		uploadedWord = "files"
	}
	skippedWord := "file"
	if skipped != 1 {
		skippedWord = "files"
	}
	failedWord := "file"
	if failed != 1 {
		failedWord = "files"
	}
	fmt.Printf("\nUpload complete: %d %s uploaded, %d %s skipped, %d %s failed\n",
		successful, uploadedWord, skipped, skippedWord, failed, failedWord)

	if failed > 0 {
		fmt.Println("\nFailed uploads:")
		for _, result := range results {
			if !result.Success {
				fmt.Printf("  %s: %v\n", result.Path, result.Error)
			}
		}
		os.Exit(1)
	}
}

func handleAdd() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, CLI.Rules.Add.Key, CLI.Rules.Add.Zone)
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
		ActionType:          1,                // Redirect
		ActionParameter1:    CLI.Rules.Add.To, // Destination URL
		ActionParameter2:    "302",            // Status code
		TriggerMatchingType: 0,                // MatchAny
		Description:         desc,
		Enabled:             true,
		Triggers: []Trigger{
			{
				Type:                0, // Url trigger
				PatternMatches:      []string{CLI.Rules.Add.From},
				PatternMatchingType: 0, // MatchAny
			},
		},
	}

	err = addEdgeRule(ctx, CLI.Rules.Add.Key, zoneID, rule)
	if err != nil {
		log.Fatalf("Error adding edge rule: %v", err)
	}

	fmt.Printf("Successfully added 302 redirect from %s to %s\n", CLI.Rules.Add.From, CLI.Rules.Add.To)
}

func handleList() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, CLI.Rules.List.Key, CLI.Rules.List.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Rules.List.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Rules.List.Zone, zoneID)

	// Get all edge rules
	rules, err := listEdgeRules(ctx, CLI.Rules.List.Key, zoneID)
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

	redirectWord := "redirect"
	if len(redirects) != 1 {
		redirectWord = "redirects"
	}
	fmt.Printf("\nFound %d 302 %s:\n", len(redirects), redirectWord)
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
	baseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, CLI.Rules.Check.Key, CLI.Rules.Check.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Rules.Check.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Rules.Check.Zone, zoneID)

	// Check rules using structured function
	result, err := checkRulesStructured(ctx, CLI.Rules.Check.Key, zoneID, CLI.Rules.Check.SkipHealth)
	if err != nil {
		log.Fatalf("Error checking rules: %v", err)
	}

	// Display results using the existing display function (it expects all issues)
	allIssues := append(result.Issues, result.Successful...)
	displayCheckResults(allIssues)
}

// setupDNSCommand handles the common setup for DNS commands
func setupDNSCommand(ctx context.Context, apiKey, zoneName string) (*PullZoneDetails, error) {
	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, apiKey, zoneName)
	if err != nil {
		return nil, fmt.Errorf("error finding pull zone '%s': %v", zoneName, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", zoneName, pullZoneID)

	// Get pull zone details to retrieve hostnames
	pullZoneDetails, err := getPullZoneDetails(ctx, apiKey, fmt.Sprintf("%d", pullZoneID))
	if err != nil {
		return nil, fmt.Errorf("error getting pull zone details: %v", err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		fmt.Println("No hostnames found for this pull zone.")
		return pullZoneDetails, nil
	}

	hostnameWord := "hostname"
	if len(pullZoneDetails.Hostnames) != 1 {
		hostnameWord = "hostnames"
	}
	fmt.Printf("Found %d %s for this pull zone:\n", len(pullZoneDetails.Hostnames), hostnameWord)
	for _, hostname := range pullZoneDetails.Hostnames {
		fmt.Printf("  - %s\n", hostname.Value)
	}

	return pullZoneDetails, nil
}

func handleDNSList() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Setup DNS command (shared logic)
	pullZoneDetails, err := setupDNSCommand(ctx, CLI.DNS.List.Key, CLI.DNS.List.Zone)
	if err != nil {
		log.Fatal(err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		return
	}

	// Get all DNS zones and search for matching records
	dnsRecords, err := findDNSRecordsForHostnames(ctx, CLI.DNS.List.Key, pullZoneDetails.Hostnames)
	if err != nil {
		log.Fatalf("Error finding DNS records: %v", err)
	}

	if len(dnsRecords) == 0 {
		fmt.Println("\nNo A or CNAME records found for these hostnames.")
		return
	}

	recordWord := "record"
	if len(dnsRecords) != 1 {
		recordWord = "records"
	}
	fmt.Printf("\nFound %d DNS %s:\n", len(dnsRecords), recordWord)

	for _, record := range dnsRecords {
		fmt.Printf("%s - %s - %s\n", record.Name, record.Type, record.Value)
	}
}

func handleCDNCheck() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, CLI.CDN.Check.Key, CLI.CDN.Check.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.CDN.Check.Zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", CLI.CDN.Check.Zone, pullZoneID)

	// Get pull zone details to check SSL configuration
	pullZoneDetails, err := getPullZoneDetails(ctx, CLI.CDN.Check.Key, fmt.Sprintf("%d", pullZoneID))
	if err != nil {
		log.Fatalf("Error getting pull zone details: %v", err)
	}

	// Check SSL configuration using structured function
	result := checkSSLConfiguration(ctx, pullZoneDetails.Hostnames)

	// Display results
	for _, success := range result.Successful {
		fmt.Println(success.Message)
	}
	for _, issue := range result.Issues {
		fmt.Println(issue.Message)
	}

	// Summary and exit code
	errorCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			errorCount++
		}
	}

	if errorCount > 0 {
		os.Exit(1)
	}
}

func handleDNSCheck() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Setup DNS command (shared logic)
	pullZoneDetails, err := setupDNSCommand(ctx, CLI.DNS.Check.Key, CLI.DNS.Check.Zone)
	if err != nil {
		log.Fatal(err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		return
	}

	// Check DNS records using structured function
	result := checkDNSRecordsStructured(ctx, CLI.DNS.Check.Key, pullZoneDetails.Hostnames)

	// Display results
	for _, success := range result.Successful {
		fmt.Println(success.Message)
	}
	for _, issue := range result.Issues {
		fmt.Println(issue.Message)
	}

	// Summary and exit code
	errorCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			errorCount++
		}
	}

	if errorCount > 0 {
		os.Exit(1)
	}
}

func handleGeneralCheck() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	fmt.Printf("Running comprehensive checks for pull zone '%s'...\n", CLI.Check.Zone)
	fmt.Println("=" + strings.Repeat("=", 60))

	// Look up pull zone by name (shared by all checks)
	pullZoneID, err := findPullZoneByName(ctx, CLI.Check.Key, CLI.Check.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.Check.Zone, err)
	}
	zoneID := fmt.Sprintf("%d", pullZoneID)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", CLI.Check.Zone, zoneID)

	// Get pull zone details (needed for DNS and SSL checks)
	pullZoneDetails, err := getPullZoneDetails(ctx, CLI.Check.Key, zoneID)
	if err != nil {
		log.Fatalf("Error getting pull zone details: %v", err)
	}

	hasErrors := false

	// 1. Rules Check
	fmt.Printf("\nRULES CHECK\n")
	fmt.Println(strings.Repeat("-", 40))

	rulesResult, err := checkRulesStructured(ctx, CLI.Check.Key, zoneID, CLI.Check.SkipHealth)
	if err != nil {
		fmt.Printf("ERROR: Failed to check rules: %v\n", err)
		hasErrors = true
	} else {
		// Display rules results using existing display function
		allIssues := append(rulesResult.Issues, rulesResult.Successful...)
		displayCheckResults(allIssues)

		// Check for errors in rules
		for _, issue := range rulesResult.Issues {
			if issue.Severity == "error" || issue.Severity == "critical" {
				hasErrors = true
				break
			}
		}
	}

	// 2. DNS Check
	fmt.Printf("\nDNS CHECK\n")
	fmt.Println(strings.Repeat("-", 40))

	if len(pullZoneDetails.Hostnames) == 0 {
		fmt.Println("No hostnames found for this pull zone.")
	} else {
		dnsResult := checkDNSRecordsStructured(ctx, CLI.Check.Key, pullZoneDetails.Hostnames)

		// Display DNS results
		for _, success := range dnsResult.Successful {
			fmt.Println(success.Message)
		}
		for _, issue := range dnsResult.Issues {
			fmt.Println(issue.Message)
			if issue.Severity == "error" {
				hasErrors = true
			}
		}

		// Show summary if no issues
		if len(dnsResult.Issues) == 0 {
			fmt.Printf("No DNS issues found! All hostname records are properly configured.\n")
		}
	}

	// 3. SSL Check
	fmt.Printf("\nSSL CHECK\n")
	fmt.Println(strings.Repeat("-", 40))

	if len(pullZoneDetails.Hostnames) == 0 {
		fmt.Println("No hostnames found for this pull zone.")
	} else {
		sslResult := checkSSLConfiguration(ctx, pullZoneDetails.Hostnames)

		// Display SSL results
		for _, success := range sslResult.Successful {
			fmt.Println(success.Message)
		}
		for _, issue := range sslResult.Issues {
			fmt.Println(issue.Message)
			if issue.Severity == "error" {
				hasErrors = true
			}
		}

		// Show summary if no issues
		if len(sslResult.Issues) == 0 {
			fmt.Printf("No SSL issues found! All hostnames have SSL properly configured.\n")
		}
	}

	// Summary
	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	if hasErrors {
		fmt.Printf("OVERALL RESULT: Issues found that require attention\n")
		os.Exit(1)
	} else {
		fmt.Printf("OVERALL RESULT: All checks passed successfully\n")
	}
}
