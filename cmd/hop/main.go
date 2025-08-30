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
	} `kong:"cmd,help='Manage CDN content'"`

	DNS struct {
		List struct {
			Key  string `kong:"required,help='Bunny CDN API key'"`
			Zone string `kong:"required,help='Pull Zone name'"`
		} `kong:"cmd,help='List DNS A and CNAME records for a pull zone'"`
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
	case "rules add":
		handleAdd()
	case "rules list":
		handleList()
	case "rules check":
		handleCheck()
	case "cdn push":
		handleCDNPush()
	case "dns list":
		handleDNSList()
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

	// Get all edge rules
	rules, err := listEdgeRules(ctx, CLI.Rules.Check.Key, zoneID)
	if err != nil {
		log.Fatalf("Error listing edge rules: %v", err)
	}

	ruleWord := "rule"
	if len(rules) != 1 {
		ruleWord = "rules"
	}
	fmt.Printf("\nRunning comprehensive redirect analysis on %d edge %s...\n", len(rules), ruleWord)
	fmt.Println("=" + strings.Repeat("=", 70))

	var allIssues []CheckIssue
	redirectMap := buildRedirectMap(rules)

	// Get pull zone details for hostname information
	pullZoneDetails, err := getPullZoneDetails(ctx, CLI.Rules.Check.Key, zoneID)
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
		allIssues = append(allIssues, checkURLHealth(ctx, rules)...)
	}

	// Display results
	displayCheckResults(allIssues)
}

func handleDNSList() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add debug flag to context
	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, CLI.DNS.List.Key, CLI.DNS.List.Zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", CLI.DNS.List.Zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", CLI.DNS.List.Zone, pullZoneID)

	// Get pull zone details to retrieve hostnames
	pullZoneDetails, err := getPullZoneDetails(ctx, CLI.DNS.List.Key, fmt.Sprintf("%d", pullZoneID))
	if err != nil {
		log.Fatalf("Error getting pull zone details: %v", err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		fmt.Println("No hostnames found for this pull zone.")
		return
	}

	hostnameWord := "hostname"
	if len(pullZoneDetails.Hostnames) != 1 {
		hostnameWord = "hostnames"
	}
	fmt.Printf("Found %d %s for this pull zone:\n", len(pullZoneDetails.Hostnames), hostnameWord)
	for _, hostname := range pullZoneDetails.Hostnames {
		fmt.Printf("  - %s\n", hostname.Value)
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
