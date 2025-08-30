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

	// Fallback: try parsing as direct array
	var dnsZones []DNSZone
	if err := json.Unmarshal(body, &dnsZones); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v (raw body: %s)", err, string(body)[:200])
	}

	return dnsZones, nil
}

func findDNSRecordsForHostnames(ctx context.Context, apiKey string, hostnames []Hostname, debug bool) ([]DNSRecordFormatted, error) {
	dnsZones, err := getAllDNSZones(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("error getting DNS zones: %v", err)
	}

	if debug {
		zoneWord := "zone"
		if len(dnsZones) != 1 {
			zoneWord = "zones"
		}
		fmt.Printf("\nDEBUG: Found %d DNS %s:\n", len(dnsZones), zoneWord)
		for _, zone := range dnsZones {
			fmt.Printf("\nZone: %s (ID: %d)\n", zone.Domain, zone.Id)
			fmt.Printf("  Records (%d):\n", len(zone.Records))
			for _, record := range zone.Records {
				if isTargetRecordType(record.Type) {
					fmt.Printf("    %s -> %s (%s, TTL: %d)\n", record.Name, record.Value, formatDNSRecordType(record.Type), record.TTL)
				}
			}
		}
		fmt.Println()
	}

	hostnameMap := createHostnameMap(hostnames)

	if debug {
		fmt.Printf("DEBUG: Looking for hostnames: %v\n", hostnames)
		fmt.Printf("DEBUG: Normalized hostname map: %v\n\n", hostnameMap)
	}

	matchingRecords := filterMatchingDNSRecords(dnsZones, hostnameMap)

	return matchingRecords, nil
}

// Wrapper function for backward compatibility with tests
func findDNSRecordsForHostnamesWithoutDebug(ctx context.Context, apiKey string, hostnames []Hostname) ([]DNSRecordFormatted, error) {
	return findDNSRecordsForHostnames(ctx, apiKey, hostnames, false)
}
