package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

func main() {
	if os.Args == nil || len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "add":
		handleAdd()
	case "list":
		handleList()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  bunny-redirect add -key API_KEY -zone ZONE_NAME -from SOURCE_URL -to DESTINATION_URL [-desc DESCRIPTION]")
	fmt.Println("  bunny-redirect list -key API_KEY -zone ZONE_NAME")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  add   Add a new 302 redirect")
	fmt.Println("  list  List all existing 302 redirects")
}

func handleAdd() {
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	apiKey := addCmd.String("key", "", "Bunny CDN API key (required)")
	pullZoneName := addCmd.String("zone", "", "Pull Zone name (required)")
	fromURL := addCmd.String("from", "", "Source URL path to redirect from (required)")
	toURL := addCmd.String("to", "", "Destination URL to redirect to (required)")
	description := addCmd.String("desc", "", "Edge rule description")

	if os.Args == nil || len(os.Args) < 3 {
		fmt.Println("Error: Not enough arguments")
		addCmd.Usage()
		os.Exit(1)
	}
	if err := addCmd.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	if *apiKey == "" {
		fmt.Println("Error: API key is required. Use -key flag.")
		addCmd.Usage()
		os.Exit(1)
	}

	if *pullZoneName == "" {
		fmt.Println("Error: Pull Zone name is required. Use -zone flag.")
		addCmd.Usage()
		os.Exit(1)
	}

	if *fromURL == "" {
		fmt.Println("Error: Source URL is required. Use -from flag.")
		addCmd.Usage()
		os.Exit(1)
	}

	if *toURL == "" {
		fmt.Println("Error: Destination URL is required. Use -to flag.")
		addCmd.Usage()
		os.Exit(1)
	}

	// Look up pull zone by name
	id, err := findPullZoneByName(*apiKey, *pullZoneName)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", *pullZoneName, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", *pullZoneName, zoneID)

	// Set default description if not provided
	desc := *description
	if desc == "" {
		desc = fmt.Sprintf("302 redirect from %s to %s", *fromURL, *toURL)
	}

	// Create the edge rule for 302 redirect using the Redirect action
	rule := EdgeRule{
		ActionType:          1,      // Redirect
		ActionParameter1:    *toURL, // Destination URL
		ActionParameter2:    "302",  // Status code
		TriggerMatchingType: 0,      // MatchAny
		Description:         desc,
		Enabled:             true,
		Triggers: []Trigger{
			{
				Type:                0, // Url trigger
				PatternMatches:      []string{*fromURL},
				PatternMatchingType: 0, // MatchAny
			},
		},
	}

	err = addEdgeRule(*apiKey, zoneID, rule)
	if err != nil {
		log.Fatalf("Error adding edge rule: %v", err)
	}

	fmt.Printf("Successfully added 302 redirect from %s to %s\n", *fromURL, *toURL)
}

func handleList() {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	apiKey := listCmd.String("key", "", "Bunny CDN API key (required)")
	pullZoneName := listCmd.String("zone", "", "Pull Zone name (required)")

	if os.Args == nil || len(os.Args) < 3 {
		fmt.Println("Error: Not enough arguments")
		listCmd.Usage()
		os.Exit(1)
	}
	if err := listCmd.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	if *apiKey == "" {
		fmt.Println("Error: API key is required. Use -key flag.")
		listCmd.Usage()
		os.Exit(1)
	}

	if *pullZoneName == "" {
		fmt.Println("Error: Pull Zone name is required. Use -zone flag.")
		listCmd.Usage()
		os.Exit(1)
	}

	// Look up pull zone by name
	id, err := findPullZoneByName(*apiKey, *pullZoneName)
	if err != nil {
		log.Fatalf("Error finding pull zone '%s': %v", *pullZoneName, err)
	}
	zoneID := fmt.Sprintf("%d", id)
	fmt.Printf("Found pull zone '%s' with ID: %s\n", *pullZoneName, zoneID)

	// Get all edge rules
	rules, err := listEdgeRules(*apiKey, zoneID)
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
