package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Bunny DNS record type for Redirect (RDR) records
const DNSRecordTypeRedirect = 5

// wwwRedirectTargets maps apex domains to their www hostname for all
// www.* hostnames attached to the pull zone (Bunny-managed hostnames excluded).
func wwwRedirectTargets(hostnames []Hostname) map[string]string {
	targets := make(map[string]string)
	for _, hostname := range hostnames {
		host := normalizeHostname(hostname.Value)
		if strings.HasSuffix(host, ".b-cdn.net") {
			continue
		}
		if strings.HasPrefix(host, "www.") {
			apex := strings.TrimPrefix(host, "www.")
			targets[apex] = host
		}
	}
	return targets
}

// isApexRecordName reports whether a record name refers to the zone apex.
// Bunny stores apex records with an empty name, but "@" and the full domain
// also occur.
func isApexRecordName(name, domain string) bool {
	normalized := normalizeHostname(strings.TrimSuffix(name, "."))
	return normalized == "" || normalized == "@" || normalized == normalizeHostname(domain)
}

// findApexRedirectRecords returns all RDR records at the zone apex
func findApexRedirectRecords(zone DNSZone) []DNSRecord {
	var records []DNSRecord
	for _, record := range zone.Records {
		if record.Type == DNSRecordTypeRedirect && isApexRecordName(record.Name, zone.Domain) {
			records = append(records, record)
		}
	}
	return records
}

// normalizeRedirectValue makes redirect destinations comparable
// (case-insensitive, ignoring a trailing slash)
func normalizeRedirectValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "/" {
		value = strings.TrimSuffix(value, "/")
	}
	return value
}

// wwwRedirectURL is the canonical destination for an apex redirect
func wwwRedirectURL(wwwHost string) string {
	return fmt.Sprintf("https://%s/", wwwHost)
}

// findDNSZoneByDomain returns the DNS zone matching the domain, or nil
func findDNSZoneByDomain(zones []DNSZone, domain string) *DNSZone {
	normalized := normalizeHostname(domain)
	for i := range zones {
		if normalizeHostname(zones[i].Domain) == normalized {
			return &zones[i]
		}
	}
	return nil
}

// checkWWWRedirectsStructured validates that every apex domain has an RDR
// record redirecting to its www hostname
func checkWWWRedirectsStructured(dnsZones []DNSZone, targets map[string]string) CheckResult {
	var result CheckResult

	if len(targets) == 0 {
		result.Successful = append(result.Successful, CheckIssue{
			Type:     "dns_www_skip",
			Severity: "info",
			Message:  "SKIP no www hostnames attached to this pull zone",
		})
		return result
	}

	for apex, wwwHost := range targets {
		zone := findDNSZoneByDomain(dnsZones, apex)
		if zone == nil {
			result.Issues = append(result.Issues, CheckIssue{
				Type:     "dns_www_missing_zone",
				Severity: "error",
				Message:  fmt.Sprintf("MISSING %s - No Bunny DNS zone found for this domain", apex),
				Details:  map[string]interface{}{"domain": apex},
			})
			continue
		}

		records := findApexRedirectRecords(*zone)
		if len(records) == 0 {
			result.Issues = append(result.Issues, CheckIssue{
				Type:     "dns_www_missing_record",
				Severity: "error",
				Message:  fmt.Sprintf("MISSING %s - No apex redirect (RDR) record to %s", apex, wwwHost),
				Details:  map[string]interface{}{"domain": apex, "expected": wwwRedirectURL(wwwHost)},
			})
			continue
		}

		expected := normalizeRedirectValue(wwwRedirectURL(wwwHost))
		matched := false
		for _, record := range records {
			if normalizeRedirectValue(record.Value) == expected {
				matched = true
				break
			}
		}

		if matched {
			result.Successful = append(result.Successful, CheckIssue{
				Type:     "dns_www_ok",
				Severity: "info",
				Message:  fmt.Sprintf("OK %s (RDR -> %s)", apex, wwwRedirectURL(wwwHost)),
				Details:  map[string]interface{}{"domain": apex},
			})
		} else {
			result.Issues = append(result.Issues, CheckIssue{
				Type:     "dns_www_wrong_destination",
				Severity: "error",
				Message: fmt.Sprintf("WRONG %s - Apex redirect points to %s, expected %s",
					apex, records[0].Value, wwwRedirectURL(wwwHost)),
				Details: map[string]interface{}{"domain": apex, "actual": records[0].Value},
			})
		}
	}

	return result
}

// addDNSRecord creates a new DNS record in a zone
func addDNSRecord(ctx context.Context, apiKey string, zoneID int64, record DNSRecord) error {
	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	url := fmt.Sprintf("https://api.bunny.net/dnszone/%d/records", zoneID)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

// updateDNSRecord updates an existing DNS record in a zone
func updateDNSRecord(ctx context.Context, apiKey string, zoneID int64, record DNSRecord) error {
	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	url := fmt.Sprintf("https://api.bunny.net/dnszone/%d/records/%d", zoneID, record.Id)

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

// deleteDNSRecord removes a DNS record from a zone
func deleteDNSRecord(ctx context.Context, apiKey string, zoneID, recordID int64) error {
	url := fmt.Sprintf("https://api.bunny.net/dnszone/%d/records/%d", zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
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

// addWWWRedirects ensures every apex domain has an RDR record to its www
// hostname. Existing apex RDR records with a different destination are
// updated in place.
func addWWWRedirects(ctx context.Context, apiKey string, dnsZones []DNSZone, targets map[string]string) error {
	for apex, wwwHost := range targets {
		zone := findDNSZoneByDomain(dnsZones, apex)
		if zone == nil {
			return fmt.Errorf("no Bunny DNS zone found for domain '%s'", apex)
		}

		destination := wwwRedirectURL(wwwHost)
		records := findApexRedirectRecords(*zone)

		if len(records) == 0 {
			fmt.Printf("Creating RDR record: %s -> %s\n", apex, destination)
			record := DNSRecord{
				Type:  DNSRecordTypeRedirect,
				Name:  "",
				Value: destination,
				TTL:   300,
			}
			if err := addDNSRecord(ctx, apiKey, zone.Id, record); err != nil {
				return fmt.Errorf("failed to create RDR record for %s: %w", apex, err)
			}
			continue
		}

		expected := normalizeRedirectValue(destination)
		alreadyConfigured := false
		for _, record := range records {
			if normalizeRedirectValue(record.Value) == expected {
				alreadyConfigured = true
				break
			}
		}
		if alreadyConfigured {
			fmt.Printf("RDR record already configured: %s -> %s\n", apex, destination)
			continue
		}

		// No matching record: repoint the first apex RDR record, leave others alone
		record := records[0]
		fmt.Printf("Updating RDR record: %s -> %s (was %s)\n", apex, destination, record.Value)
		record.Value = destination
		if err := updateDNSRecord(ctx, apiKey, zone.Id, record); err != nil {
			return fmt.Errorf("failed to update RDR record for %s: %w", apex, err)
		}
	}

	return nil
}

// removeWWWRedirects deletes apex RDR records that point to the www hostname.
// Records with other destinations are left untouched and reported.
func removeWWWRedirects(ctx context.Context, apiKey string, dnsZones []DNSZone, targets map[string]string) (int, error) {
	deleted := 0
	for apex, wwwHost := range targets {
		zone := findDNSZoneByDomain(dnsZones, apex)
		if zone == nil {
			fmt.Printf("No Bunny DNS zone found for domain '%s', skipping\n", apex)
			continue
		}

		expected := normalizeRedirectValue(wwwRedirectURL(wwwHost))
		for _, record := range findApexRedirectRecords(*zone) {
			if normalizeRedirectValue(record.Value) != expected {
				fmt.Printf("Keeping RDR record on %s (points to %s, not %s)\n", apex, record.Value, wwwRedirectURL(wwwHost))
				continue
			}
			fmt.Printf("Removing RDR record: %s -> %s\n", apex, record.Value)
			if err := deleteDNSRecord(ctx, apiKey, zone.Id, record.Id); err != nil {
				return deleted, fmt.Errorf("failed to delete RDR record for %s: %w", apex, err)
			}
			deleted++
		}
	}

	return deleted, nil
}
