package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var debugFlag bool

// Global flag variables
var (
	// Common flags
	apiKey     string
	zone       string
	skipHealth bool

	// Rules add specific
	fromURL     string
	toURL       string
	description string

	// CDN push specific
	localDir   string
	purgeCache bool

	// Stats specific
	days     int
	hourly   bool
	detailed bool

	// Minify specific
	minifyCacheDir string
	minifyForce    bool
	minifyExclude  []string

	// Slash redirect specific
	slashHost string
)

// createDebugContext creates a context with debug flag from global CLI
func createDebugContext(baseCtx context.Context) context.Context {
	return context.WithValue(baseCtx, struct{ key string }{"debug"}, debugFlag)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "hop",
	Short: "A Go command-line tool to manage 302 redirects, content, and statistics in Bunny CDN pull zones",
	Long:  "A Go command-line tool to manage 302 redirects, content, and statistics in Bunny CDN pull zones.",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug output")

	// Add all commands to root
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(rulesCmd)
	rootCmd.AddCommand(cdnCmd)
	rootCmd.AddCommand(dnsCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(minifyCmd)
	rootCmd.AddCommand(securityCmd)

	// Setup rules subcommands
	rulesCmd.AddCommand(rulesAddCmd)
	rulesCmd.AddCommand(rulesListCmd)
	rulesCmd.AddCommand(rulesCheckCmd)
	rulesCmd.AddCommand(rulesSlashAddCmd)
	rulesCmd.AddCommand(rulesSlashRemoveCmd)

	// Setup CDN subcommands
	cdnCmd.AddCommand(cdnPushCmd)
	cdnCmd.AddCommand(cdnCheckCmd)

	// Setup DNS subcommands
	dnsCmd.AddCommand(dnsListCmd)
	dnsCmd.AddCommand(dnsCheckCmd)

	// Setup stats subcommands
	statsCmd.AddCommand(statsShowCmd)

	// Setup security subcommands
	securityCmd.AddCommand(securityCheckCmd)
	securityCmd.AddCommand(securityFixCmd)

	// Setup flags for check command
	checkCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	checkCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	checkCmd.Flags().BoolVar(&skipHealth, "skip-health", false, "Skip HTTP health checks for faster execution")
	_ = checkCmd.MarkFlagRequired("key")
	_ = checkCmd.MarkFlagRequired("zone")

	// Setup flags for rules add command
	rulesAddCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	rulesAddCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	rulesAddCmd.Flags().StringVar(&fromURL, "from", "", "Source URL path to redirect from (required)")
	rulesAddCmd.Flags().StringVar(&toURL, "to", "", "Destination URL to redirect to (required)")
	rulesAddCmd.Flags().StringVar(&description, "desc", "", "Edge rule description")
	_ = rulesAddCmd.MarkFlagRequired("key")
	_ = rulesAddCmd.MarkFlagRequired("zone")
	_ = rulesAddCmd.MarkFlagRequired("from")
	_ = rulesAddCmd.MarkFlagRequired("to")

	// Setup flags for rules list command
	rulesListCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	rulesListCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = rulesListCmd.MarkFlagRequired("key")
	_ = rulesListCmd.MarkFlagRequired("zone")

	// Setup flags for rules check command
	rulesCheckCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	rulesCheckCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	rulesCheckCmd.Flags().BoolVar(&skipHealth, "skip-health", false, "Skip HTTP health checks for faster execution")
	_ = rulesCheckCmd.MarkFlagRequired("key")
	_ = rulesCheckCmd.MarkFlagRequired("zone")

	// Setup flags for CDN push command
	cdnPushCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	cdnPushCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	cdnPushCmd.Flags().StringVar(&localDir, "from", "", "Local directory path to upload from (required)")
	_ = cdnPushCmd.MarkFlagRequired("key")
	_ = cdnPushCmd.MarkFlagRequired("zone")
	_ = cdnPushCmd.MarkFlagRequired("from")
	cdnPushCmd.Flags().BoolVar(&purgeCache, "purge", false, "Purge pull zone cache after upload")

	// Setup flags for CDN check command
	cdnCheckCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	cdnCheckCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = cdnCheckCmd.MarkFlagRequired("key")
	_ = cdnCheckCmd.MarkFlagRequired("zone")

	// Setup flags for DNS list command
	dnsListCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	dnsListCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = dnsListCmd.MarkFlagRequired("key")
	_ = dnsListCmd.MarkFlagRequired("zone")

	// Setup flags for DNS check command
	dnsCheckCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	dnsCheckCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = dnsCheckCmd.MarkFlagRequired("key")
	_ = dnsCheckCmd.MarkFlagRequired("zone")

	// Setup flags for stats show command
	statsShowCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	statsShowCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	statsShowCmd.Flags().IntVar(&days, "days", 7, "Number of days to retrieve statistics for (1-30)")
	statsShowCmd.Flags().BoolVar(&hourly, "hourly", false, "Show hourly breakdown instead of daily")
	statsShowCmd.Flags().BoolVar(&detailed, "detailed", false, "Show detailed charts and geographic distribution")
	_ = statsShowCmd.MarkFlagRequired("key")
	_ = statsShowCmd.MarkFlagRequired("zone")

	// Setup flags for minify command
	minifyCmd.Flags().StringVar(&minifyCacheDir, "cache", ".minify-cache", "Cache directory for WebP conversions")
	minifyCmd.Flags().BoolVar(&minifyForce, "force", false, "Force reprocessing of all files")
	minifyCmd.Flags().StringSliceVar(&minifyExclude, "exclude", []string{"newsletter/**"}, "Glob patterns to exclude")

	// Setup flags for security check command
	securityCheckCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	securityCheckCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = securityCheckCmd.MarkFlagRequired("key")
	_ = securityCheckCmd.MarkFlagRequired("zone")

	// Setup flags for security fix command
	securityFixCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	securityFixCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = securityFixCmd.MarkFlagRequired("key")
	_ = securityFixCmd.MarkFlagRequired("zone")

	// Setup flags for rules slash-add command
	rulesSlashAddCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	rulesSlashAddCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	rulesSlashAddCmd.Flags().StringVar(&slashHost, "host", "", "Destination hostname for redirects (required)")
	_ = rulesSlashAddCmd.MarkFlagRequired("key")
	_ = rulesSlashAddCmd.MarkFlagRequired("zone")
	_ = rulesSlashAddCmd.MarkFlagRequired("host")

	// Setup flags for rules slash-remove command (only needs key and zone)
	rulesSlashRemoveCmd.Flags().StringVarP(&apiKey, "key", "k", "", "Bunny CDN API key (required)")
	rulesSlashRemoveCmd.Flags().StringVarP(&zone, "zone", "z", "", "Pull Zone name (required)")
	_ = rulesSlashRemoveCmd.MarkFlagRequired("key")
	_ = rulesSlashRemoveCmd.MarkFlagRequired("zone")
}

// Global check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run all checks (rules, DNS, SSL) for a pull zone",
	Run: func(cmd *cobra.Command, args []string) {
		handleGeneralCheck()
	},
}

// Rules command group
var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage redirect rules",
	Long:  "Manage redirect rules in Bunny CDN pull zones",
}

var rulesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new 302 redirect",
	Run: func(cmd *cobra.Command, args []string) {
		handleAdd()
	},
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all existing 302 redirects",
	Run: func(cmd *cobra.Command, args []string) {
		handleList()
	},
}

var rulesCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check redirect rules for potential issues",
	Run: func(cmd *cobra.Command, args []string) {
		handleCheck()
	},
}

var rulesSlashAddCmd = &cobra.Command{
	Use:   "slash-add",
	Short: "Add trailing slash redirects for extensionless URLs",
	Long:  "Creates edge rules to redirect URLs without trailing slashes to include them (e.g., /about -> /about/). Only affects extensionless paths, not files like .png, .js, etc.",
	Run: func(cmd *cobra.Command, args []string) {
		handleSlashAdd()
	},
}

var rulesSlashRemoveCmd = &cobra.Command{
	Use:   "slash-remove",
	Short: "Remove trailing slash redirect rules",
	Long:  "Removes the edge rules created by slash-add.",
	Run: func(cmd *cobra.Command, args []string) {
		handleSlashRemove()
	},
}

// CDN command group
var cdnCmd = &cobra.Command{
	Use:   "cdn",
	Short: "Manage CDN content",
	Long:  "Manage CDN content in Bunny CDN pull zones",
}

var cdnPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push files from local directory to CDN storage",
	Run: func(cmd *cobra.Command, args []string) {
		handleCDNPush()
	},
}

var cdnCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check SSL configuration for all pull zone hostnames",
	Run: func(cmd *cobra.Command, args []string) {
		handleCDNCheck()
	},
}

// DNS command group
var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage DNS records",
	Long:  "Manage DNS records for Bunny CDN pull zones",
}

var dnsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List DNS A and CNAME records for a pull zone",
	Run: func(cmd *cobra.Command, args []string) {
		handleDNSList()
	},
}

var dnsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check DNS records exist for pull zone hostnames",
	Run: func(cmd *cobra.Command, args []string) {
		handleDNSCheck()
	},
}

// Stats command group
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View pull zone statistics",
	Long:  "View pull zone statistics from Bunny CDN",
}

var statsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show usage statistics for a pull zone",
	Run: func(cmd *cobra.Command, args []string) {
		handleStatsShow()
	},
}

// Minify command
var minifyCmd = &cobra.Command{
	Use:   "minify <source> <target>",
	Short: "Minify HTML/CSS/JS and optimize images to WebP",
	Long: `Minify HTML, CSS, JavaScript, SVG, and XML files.
Converts images to WebP with responsive srcset.
Converts TTF fonts to WOFF2.
Inlines critical CSS for faster page loads.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		handleMinify(args[0], args[1])
	},
}

// Security command group
var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Check security headers configuration",
	Long:  "Check that recommended security HTTP headers are configured as edge rules",
}

var securityCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check security headers are configured as edge rules",
	Run: func(cmd *cobra.Command, args []string) {
		handleSecurityCheck()
	},
}

var securityFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Add missing security headers as edge rules",
	Run: func(cmd *cobra.Command, args []string) {
		handleSecurityFix()
	},
}

func handleCDNPush() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Verify local directory exists
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		log.Fatalf("Local directory '%s' does not exist", localDir)
	}

	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", zone, pullZoneID)

	// Find associated storage zone
	storageZone, err := getStorageZoneByPullZone(ctx, apiKey, pullZoneID)
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

	if purgeCache && successful > 0 {
		fmt.Print("Purging pull zone cache... ")
		if err := purgePullZoneCache(ctx, apiKey, pullZoneID); err != nil {
			log.Fatalf("Error purging cache: %v", err)
		}
		fmt.Println("done")
	}
}

func handleAdd() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Get existing rules to check for matching destination
	existingRules, err := listEdgeRules(ctx, apiKey, zoneID)
	if err != nil {
		log.Fatalf("Error listing existing rules: %v", err)
	}

	// Search for existing rule with same destination
	var matchingRule *EdgeRuleResponse
	for i, rule := range existingRules {
		if rule.ActionType == 1 && rule.ActionParameter1 == toURL && rule.ActionParameter2 == "302" {
			matchingRule = &existingRules[i]
			break
		}
	}

	if matchingRule != nil {
		// Check if pattern already exists
		if len(matchingRule.Triggers) > 0 {
			for _, pattern := range matchingRule.Triggers[0].PatternMatches {
				if pattern == fromURL {
					fmt.Printf("Pattern '%s' already exists in rule for destination '%s'\n", fromURL, toURL)
					return
				}
			}
		}

		// Add new pattern to existing rule
		newPatterns := append(matchingRule.Triggers[0].PatternMatches, fromURL)
		rule := EdgeRule{
			Guid:                matchingRule.Guid,
			ActionType:          matchingRule.ActionType,
			ActionParameter1:    matchingRule.ActionParameter1,
			ActionParameter2:    matchingRule.ActionParameter2,
			TriggerMatchingType: matchingRule.TriggerMatchingType,
			Description:         matchingRule.Description,
			Enabled:             matchingRule.Enabled,
			Triggers: []Trigger{
				{
					Type:                matchingRule.Triggers[0].Type,
					PatternMatches:      newPatterns,
					PatternMatchingType: matchingRule.Triggers[0].PatternMatchingType,
				},
			},
		}

		err = addEdgeRule(ctx, apiKey, zoneID, rule)
		if err != nil {
			log.Fatalf("Error updating edge rule: %v", err)
		}

		fmt.Printf("Added pattern '%s' to existing rule for destination '%s'\n", fromURL, toURL)
		fmt.Printf("Rule now has %d patterns\n", len(newPatterns))
	} else {
		// Set default description if not provided
		desc := description
		if desc == "" {
			desc = fmt.Sprintf("302 redirect to %s", toURL)
		}

		// Create new edge rule for 302 redirect
		rule := EdgeRule{
			ActionType:          1,     // Redirect
			ActionParameter1:    toURL, // Destination URL
			ActionParameter2:    "302", // Status code
			TriggerMatchingType: 0,     // MatchAny
			Description:         desc,
			Enabled:             true,
			Triggers: []Trigger{
				{
					Type:                0, // Url trigger
					PatternMatches:      []string{fromURL},
					PatternMatchingType: 0, // MatchAny
				},
			},
		}

		err = addEdgeRule(ctx, apiKey, zoneID, rule)
		if err != nil {
			log.Fatalf("Error adding edge rule: %v", err)
		}

		fmt.Printf("Created new 302 redirect from '%s' to '%s'\n", fromURL, toURL)
	}
}

func handleList() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Get all edge rules
	rules, err := listEdgeRules(ctx, apiKey, zoneID)
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

		// Extract all source URLs from triggers
		if len(redirect.Triggers) > 0 && len(redirect.Triggers[0].PatternMatches) > 0 {
			patterns := redirect.Triggers[0].PatternMatches
			if len(patterns) == 1 {
				fmt.Printf("   From: %s\n", patterns[0])
			} else {
				fmt.Printf("   From: (%d patterns)\n", len(patterns))
				for _, pattern := range patterns {
					fmt.Printf("     - %s\n", pattern)
				}
			}
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
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Check rules using structured function
	result, err := checkRulesStructured(ctx, apiKey, zoneID, skipHealth)
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
	pullZoneDetails, err := setupDNSCommand(ctx, apiKey, zone)
	if err != nil {
		log.Fatal(err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		return
	}

	// Get all DNS zones and search for matching records
	dnsRecords, err := findDNSRecordsForHostnames(ctx, apiKey, pullZoneDetails.Hostnames)
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
	pullZoneID, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", zone, pullZoneID)

	// Get pull zone details to check SSL configuration
	pullZoneDetails, err := getPullZoneDetails(ctx, apiKey, fmt.Sprintf("%d", pullZoneID))
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
	pullZoneDetails, err := setupDNSCommand(ctx, apiKey, zone)
	if err != nil {
		log.Fatal(err)
	}

	if len(pullZoneDetails.Hostnames) == 0 {
		return
	}

	// Check DNS records using structured function
	result := checkDNSRecordsStructured(ctx, apiKey, pullZoneDetails.Hostnames)

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

	fmt.Printf("Running comprehensive checks for pull zone '%s'...\n", zone)
	fmt.Println("=" + strings.Repeat("=", 60))

	// Look up pull zone by name (shared by all checks)
	pullZoneID, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", pullZoneID)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Get pull zone details (needed for DNS and SSL checks)
	pullZoneDetails, err := getPullZoneDetails(ctx, apiKey, zoneID)
	if err != nil {
		log.Fatalf("Error getting pull zone details: %v", err)
	}

	hasErrors := false

	// 1. Rules Check
	fmt.Printf("\nRULES CHECK\n")
	fmt.Println(strings.Repeat("-", 40))

	rulesResult, err := checkRulesStructured(ctx, apiKey, zoneID, skipHealth)
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
		dnsResult := checkDNSRecordsStructured(ctx, apiKey, pullZoneDetails.Hostnames)

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

func handleStatsShow() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Validate days parameter
	if days < 1 || days > 30 {
		log.Fatalf("Days must be between 1 and 30, got: %d", days)
	}

	// Look up pull zone by name
	pullZoneID, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	fmt.Printf("Found pull zone '%s' with ID: %d\n", zone, pullZoneID)

	// Set date range
	dateTo := time.Now()
	dateFrom := dateTo.AddDate(0, 0, -days)

	// Configure statistics parameters
	params := StatisticsParams{
		DateFrom:                          dateFrom,
		DateTo:                            dateTo,
		PullZoneID:                        pullZoneID,
		Hourly:                            hourly,
		LoadBandwidthUsed:                 true,
		LoadRequestsServed:                true,
		LoadGeographicTrafficDistribution: detailed,
	}

	// Fetch statistics
	stats, err := getStatistics(ctx, apiKey, params)
	if err != nil {
		log.Fatalf("Error fetching statistics: %v", err)
	}

	// Display results
	fmt.Printf("\n")
	fmt.Printf("STATISTICS FOR PULL ZONE: %s\n", zone)
	fmt.Printf("Date Range: %s to %s (%d days)\n",
		dateFrom.Format("2006-01-02"),
		dateTo.Format("2006-01-02"),
		days)
	fmt.Printf("Granularity: %s\n", map[bool]string{true: "Hourly", false: "Daily"}[hourly])
	fmt.Println(strings.Repeat("=", 70))

	// Show summary
	displayStatisticsSummary(stats)

	// Show detailed information if requested
	if detailed {
		if len(stats.BandwidthUsedChart) > 0 {
			displayBandwidthChart(stats.BandwidthUsedChart, "Bandwidth Usage Over Time")
		}

		if len(stats.RequestsServedChart) > 0 {
			displayRequestsChart(stats.RequestsServedChart, "Requests Served Over Time")
		}

		if len(stats.GeoTrafficDistribution) > 0 {
			displayGeographicDistribution(stats.GeoTrafficDistribution)
		}
	}
}

func handleMinify(source, target string) {
	// Verify source directory exists
	if _, err := os.Stat(source); os.IsNotExist(err) {
		log.Fatalf("Source directory '%s' does not exist", source)
	}

	if err := minifyCommand(source, target, minifyCacheDir, minifyForce, minifyExclude); err != nil {
		log.Fatalf("Minify failed: %v", err)
	}
}

func handleSecurityCheck() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Check security headers
	result, err := checkSecurityHeaders(ctx, apiKey, zoneID)
	if err != nil {
		log.Fatalf("Error checking security headers: %v", err)
	}

	// Display results
	displaySecurityResults(result)

	// Exit with error if any critical headers are missing
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			os.Exit(1)
		}
	}
}

func handleSecurityFix() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Fix missing security headers
	err = fixSecurityHeaders(ctx, apiKey, zoneID)
	if err != nil {
		log.Fatalf("Error fixing security headers: %v", err)
	}
}

func handleSlashAdd() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Remove existing slash-add rules first (for idempotency)
	_, err = deleteSlashRules(ctx, apiKey, zoneID, []string{
		SlashAddNoQueryID,
		SlashAddWithQueryID,
	})
	if err != nil {
		log.Fatalf("Error removing existing rules: %v", err)
	}

	// Create new slash-add rules
	err = createSlashAddRules(ctx, apiKey, zoneID, slashHost)
	if err != nil {
		log.Fatalf("Error creating slash-add rules: %v", err)
	}

	fmt.Println("\nTrailing slash redirect rules created successfully.")
	fmt.Printf("URLs without extensions will redirect to include trailing slash.\n")
	fmt.Printf("Destination host: %s\n", slashHost)
}

func handleSlashRemove() {
	baseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx := createDebugContext(baseCtx)

	// Look up pull zone by name
	id, err := findPullZoneByName(ctx, apiKey, zone)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", zone, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", zone, zoneID)

	// Remove slash-add rules
	deleted, err := deleteSlashRules(ctx, apiKey, zoneID, []string{
		SlashAddNoQueryID,
		SlashAddWithQueryID,
	})
	if err != nil {
		log.Fatalf("Error removing slash rules: %v", err)
	}

	if deleted == 0 {
		fmt.Println("No trailing slash rules found to remove.")
	} else {
		fmt.Printf("\nRemoved %d trailing slash rule(s).\n", deleted)
	}
}
