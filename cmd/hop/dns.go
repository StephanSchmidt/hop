package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// debug checks if debug mode is enabled in the context
func debug(ctx context.Context) bool {
	if val := ctx.Value(struct{ key string }{"debug"}); val != nil {
		if debugEnabled, ok := val.(bool); ok {
			return debugEnabled
		}
	}
	return false
}

type DNSZone struct {
	Id      int64       `json:"Id"`
	Domain  string      `json:"Domain"`
	Records []DNSRecord `json:"Records"`
}

type DNSRecord struct {
	Id    int64  `json:"Id"`
	Type  int    `json:"Type"`
	Name  string `json:"Name"`
	Value string `json:"Value"`
	TTL   int    `json:"Ttl"`
}

type DNSRecordFormatted struct {
	Name  string
	Type  string
	Value string
	TTL   int
}

type DNSValidationResult struct {
	Hostname    string
	HasRecord   bool
	RecordType  string
	RecordValue string
}

// Side effect free functions

func formatDNSRecordType(recordType int) string {
	switch recordType {
	case 0:
		return "A"
	case 1:
		return "AAAA"
	case 2:
		return "CNAME"
	case 3:
		return "TXT"
	case 4:
		return "MX"
	case 5:
		return "RDR"
	case 7:
		return "PZ"
	case 8:
		return "SRV"
	case 9:
		return "CAA"
	case 10:
		return "PTR"
	case 11:
		return "SCR"
	case 12:
		return "NS"
	default:
		return fmt.Sprintf("TYPE%d", recordType)
	}
}

func isTargetRecordType(recordType int) bool {
	return recordType == 0 || recordType == 2 // A or CNAME
}

func normalizeHostname(hostname string) string {
	return strings.ToLower(hostname)
}

func createHostnameMap(hostnames []Hostname) map[string]bool {
	hostnameMap := make(map[string]bool)
	for _, hostname := range hostnames {
		hostnameMap[normalizeHostname(hostname.Value)] = true
	}
	return hostnameMap
}

func filterMatchingDNSRecords(dnsZones []DNSZone, hostnameMap map[string]bool) []DNSRecordFormatted {
	var matchingRecords []DNSRecordFormatted

	// Go through ALL DNS zones and ALL A/CNAME records
	for _, zone := range dnsZones {
		for _, record := range zone.Records {
			// Only look at A and CNAME records
			if !isTargetRecordType(record.Type) {
				continue
			}

			// Handle both full domain names and relative names
			recordNames := []string{record.Name} // Always check the name as-is

			// If it's a relative name (doesn't contain dots or is different from zone), also try full name
			if record.Name != zone.Domain && !strings.Contains(record.Name, ".") {
				fullName := record.Name + "." + zone.Domain
				recordNames = append(recordNames, fullName)
			}

			// Check all possible names for this record
			for _, recordName := range recordNames {
				normalizedRecordName := normalizeHostname(recordName)
				if hostnameMap[normalizedRecordName] {
					matchingRecords = append(matchingRecords, DNSRecordFormatted{
						Name:  recordName,
						Type:  formatDNSRecordType(record.Type),
						Value: record.Value,
						TTL:   record.TTL,
					})
					break // Only add once per record
				}
			}
		}
	}

	return matchingRecords
}

// Side effect functions (HTTP calls)

type DNSZoneListResponse struct {
	Items        []DNSZone `json:"Items"`
	CurrentPage  int       `json:"CurrentPage"`
	TotalItems   int       `json:"TotalItems"`
	HasMoreItems bool      `json:"HasMoreItems"`
}

func getAllDNSZones(ctx context.Context, apiKey string) ([]DNSZone, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.bunny.net/dnszone", nil)
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

	// Try parsing as paginated response first
	var listResponse DNSZoneListResponse
	if err := json.Unmarshal(body, &listResponse); err == nil {
		return listResponse.Items, nil
	}

	// Fallback: try parsing as direct array (Note: arrays can't use strictUnmarshal)
	var dnsZones []DNSZone
	if err := json.Unmarshal(body, &dnsZones); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v (raw body: %s)", err, string(body)[:200])
	}

	return dnsZones, nil
}

func findDNSRecordsForHostnames(ctx context.Context, apiKey string, hostnames []Hostname) ([]DNSRecordFormatted, error) {
	dnsZones, err := getAllDNSZones(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("error getting DNS zones: %v", err)
	}

	hostnameMap := createHostnameMap(hostnames)

	if debug(ctx) {
		printDNSZonesSummary(dnsZones)
		printHostnameLookup(hostnames)
	}

	matchingRecords := filterMatchingDNSRecords(dnsZones, hostnameMap)

	return matchingRecords, nil
}

// printDNSZonesSummary prints debug information about DNS zones
func printDNSZonesSummary(dnsZones []DNSZone) {
	zoneWord := "zone"
	if len(dnsZones) != 1 {
		zoneWord = "zones"
	}
	fmt.Printf("\nDEBUG: Found %d DNS %s:\n", len(dnsZones), zoneWord)

	for _, zone := range dnsZones {
		targetRecords := 0
		for _, record := range zone.Records {
			if isTargetRecordType(record.Type) {
				targetRecords++
			}
		}
		fmt.Printf("  %s (%d A/CNAME records)\n", zone.Domain, targetRecords)
	}
	fmt.Println()
}

// printHostnameLookup prints debug information about hostname matching
func printHostnameLookup(hostnames []Hostname) {
	fmt.Printf("DEBUG: Looking for these pull zone hostnames:\n")
	for _, hostname := range hostnames {
		fmt.Printf("  - %s\n", hostname.Value)
	}
	fmt.Println()
}

// checkDNSRecordsForHostnames validates that DNS records exist for all hostnames
func checkDNSRecordsForHostnames(ctx context.Context, apiKey string, hostnames []Hostname) []DNSValidationResult {
	dnsZones, err := getAllDNSZones(ctx, apiKey)
	if err != nil {
		// Return all failed results if we can't get DNS zones
		results := make([]DNSValidationResult, len(hostnames))
		for i, hostname := range hostnames {
			results[i] = DNSValidationResult{
				Hostname:  hostname.Value,
				HasRecord: false,
			}
		}
		return results
	}

	hostnameMap := createHostnameMap(hostnames)

	if debug(ctx) {
		printDNSZonesSummary(dnsZones)
		printHostnameLookup(hostnames)
	}

	matchingRecords := filterMatchingDNSRecords(dnsZones, hostnameMap)

	// Create validation results for each hostname
	results := make([]DNSValidationResult, len(hostnames))
	for i, hostname := range hostnames {
		result := DNSValidationResult{
			Hostname:  hostname.Value,
			HasRecord: false,
		}

		// Find matching record for this hostname
		normalizedHostname := normalizeHostname(hostname.Value)
		for _, record := range matchingRecords {
			if normalizeHostname(record.Name) == normalizedHostname {
				result.HasRecord = true
				result.RecordType = record.Type
				result.RecordValue = record.Value
				break
			}
		}

		results[i] = result
	}

	return results
}

// checkDNSRecordsStructured validates DNS records and returns structured results
func checkDNSRecordsStructured(ctx context.Context, apiKey string, hostnames []Hostname) CheckResult {
	var result CheckResult

	validationResults := checkDNSRecordsForHostnames(ctx, apiKey, hostnames)

	for _, validation := range validationResults {
		// Skip .b-cdn.net hostnames as they're managed by Bunny
		if strings.HasSuffix(validation.Hostname, ".b-cdn.net") {
			result.Successful = append(result.Successful, CheckIssue{
				Type:     "dns_skip",
				Severity: "info",
				Message:  fmt.Sprintf("SKIP %s (Bunny-managed)", validation.Hostname),
				Details:  map[string]interface{}{"hostname": validation.Hostname},
			})
			continue
		}

		if !validation.HasRecord {
			result.Issues = append(result.Issues, CheckIssue{
				Type:     "dns_missing_record",
				Severity: "error",
				Message:  fmt.Sprintf("MISSING %s - No DNS record found", validation.Hostname),
				Details:  map[string]interface{}{"hostname": validation.Hostname},
			})
		} else {
			result.Successful = append(result.Successful, CheckIssue{
				Type:     "dns_ok",
				Severity: "info",
				Message:  fmt.Sprintf("OK %s (%s -> %s)", validation.Hostname, validation.RecordType, validation.RecordValue),
				Details: map[string]interface{}{
					"hostname":     validation.Hostname,
					"record_type":  validation.RecordType,
					"record_value": validation.RecordValue,
				},
			})
		}
	}

	return result
}
