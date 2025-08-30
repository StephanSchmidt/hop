package main

import (
	"reflect"
	"testing"
)

func TestFormatDNSRecordType(t *testing.T) {
	tests := []struct {
		name       string
		recordType int
		expected   string
	}{
		{
			name:       "A record",
			recordType: 0,
			expected:   "A",
		},
		{
			name:       "CNAME record",
			recordType: 2,
			expected:   "CNAME",
		},
		{
			name:       "Unknown record type",
			recordType: 999,
			expected:   "TYPE999",
		},
		{
			name:       "MX record",
			recordType: 15,
			expected:   "TYPE15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDNSRecordType(tt.recordType)
			if result != tt.expected {
				t.Errorf("formatDNSRecordType(%d) = %q, want %q", tt.recordType, result, tt.expected)
			}
		})
	}
}

func TestIsTargetRecordType(t *testing.T) {
	tests := []struct {
		name       string
		recordType int
		expected   bool
	}{
		{
			name:       "A record should be target",
			recordType: 0,
			expected:   true,
		},
		{
			name:       "CNAME record should be target",
			recordType: 2,
			expected:   true,
		},
		{
			name:       "MX record should not be target",
			recordType: 15,
			expected:   false,
		},
		{
			name:       "NS record should not be target",
			recordType: 12,
			expected:   false,
		},
		{
			name:       "Unknown record should not be target",
			recordType: 999,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTargetRecordType(tt.recordType)
			if result != tt.expected {
				t.Errorf("isTargetRecordType(%d) = %v, want %v", tt.recordType, result, tt.expected)
			}
		})
	}
}

func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected string
	}{
		{
			name:     "lowercase hostname unchanged",
			hostname: "example.com",
			expected: "example.com",
		},
		{
			name:     "uppercase hostname lowercased",
			hostname: "EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "mixed case hostname lowercased",
			hostname: "ExAmPlE.CoM",
			expected: "example.com",
		},
		{
			name:     "empty string",
			hostname: "",
			expected: "",
		},
		{
			name:     "subdomain with mixed case",
			hostname: "WWW.ExAmPlE.CoM",
			expected: "www.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("normalizeHostname(%q) = %q, want %q", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestCreateHostnameMap(t *testing.T) {
	tests := []struct {
		name      string
		hostnames []Hostname
		expected  map[string]bool
	}{
		{
			name:      "empty hostnames",
			hostnames: []Hostname{},
			expected:  map[string]bool{},
		},
		{
			name: "single hostname",
			hostnames: []Hostname{
				{Id: 1, Value: "example.com"},
			},
			expected: map[string]bool{
				"example.com": true,
			},
		},
		{
			name: "multiple hostnames",
			hostnames: []Hostname{
				{Id: 1, Value: "example.com"},
				{Id: 2, Value: "test.example.com"},
			},
			expected: map[string]bool{
				"example.com":      true,
				"test.example.com": true,
			},
		},
		{
			name: "hostnames with mixed case",
			hostnames: []Hostname{
				{Id: 1, Value: "EXAMPLE.COM"},
				{Id: 2, Value: "Test.Example.COM"},
			},
			expected: map[string]bool{
				"example.com":      true,
				"test.example.com": true,
			},
		},
		{
			name: "duplicate hostnames (after normalization)",
			hostnames: []Hostname{
				{Id: 1, Value: "example.com"},
				{Id: 2, Value: "EXAMPLE.COM"},
			},
			expected: map[string]bool{
				"example.com": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createHostnameMap(tt.hostnames)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("createHostnameMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterMatchingDNSRecords(t *testing.T) {
	tests := []struct {
		name        string
		dnsZones    []DNSZone
		hostnameMap map[string]bool
		expected    []DNSRecordFormatted
	}{
		{
			name:        "no DNS zones",
			dnsZones:    []DNSZone{},
			hostnameMap: map[string]bool{"example.com": true},
			expected:    []DNSRecordFormatted{},
		},
		{
			name: "no matching records",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "different.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "different.com", Value: "1.2.3.4", TTL: 300},
					},
				},
			},
			hostnameMap: map[string]bool{"example.com": true},
			expected:    []DNSRecordFormatted{},
		},
		{
			name: "single A record match",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "example.com", Value: "1.2.3.4", TTL: 300},
					},
				},
			},
			hostnameMap: map[string]bool{"example.com": true},
			expected: []DNSRecordFormatted{
				{Name: "example.com", Type: "A", Value: "1.2.3.4", TTL: 300},
			},
		},
		{
			name: "single CNAME record match",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 2, Name: "www.example.com", Value: "example.com", TTL: 3600},
					},
				},
			},
			hostnameMap: map[string]bool{"www.example.com": true},
			expected: []DNSRecordFormatted{
				{Name: "www.example.com", Type: "CNAME", Value: "example.com", TTL: 3600},
			},
		},
		{
			name: "multiple matching records",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "example.com", Value: "1.2.3.4", TTL: 300},
						{Id: 2, Type: 2, Name: "www.example.com", Value: "example.com", TTL: 3600},
						{Id: 3, Type: 15, Name: "example.com", Value: "mail.example.com", TTL: 3600}, // MX record - should be filtered out
					},
				},
			},
			hostnameMap: map[string]bool{"example.com": true, "www.example.com": true},
			expected: []DNSRecordFormatted{
				{Name: "example.com", Type: "A", Value: "1.2.3.4", TTL: 300},
				{Name: "www.example.com", Type: "CNAME", Value: "example.com", TTL: 3600},
			},
		},
		{
			name: "case insensitive matching",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "EXAMPLE.COM", Value: "1.2.3.4", TTL: 300},
					},
				},
			},
			hostnameMap: map[string]bool{"example.com": true},
			expected: []DNSRecordFormatted{
				{Name: "EXAMPLE.COM", Type: "A", Value: "1.2.3.4", TTL: 300},
			},
		},
		{
			name: "multiple DNS zones",
			dnsZones: []DNSZone{
				{
					Id:     1,
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "example.com", Value: "1.2.3.4", TTL: 300},
					},
				},
				{
					Id:     2,
					Domain: "test.com",
					Records: []DNSRecord{
						{Id: 2, Type: 0, Name: "test.com", Value: "5.6.7.8", TTL: 600},
					},
				},
			},
			hostnameMap: map[string]bool{"example.com": true, "test.com": true},
			expected: []DNSRecordFormatted{
				{Name: "example.com", Type: "A", Value: "1.2.3.4", TTL: 300},
				{Name: "test.com", Type: "A", Value: "5.6.7.8", TTL: 600},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterMatchingDNSRecords(tt.dnsZones, tt.hostnameMap)

			// Handle nil vs empty slice comparison
			if len(result) == 0 && len(tt.expected) == 0 {
				return // Both are effectively empty
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("filterMatchingDNSRecords() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterMatchingDNSRecordsIntegration(t *testing.T) {
	// Test the integration of multiple side effect free functions together
	hostnames := []Hostname{
		{Id: 1, Value: "Example.COM"},     // Mixed case
		{Id: 2, Value: "www.example.com"}, // Lowercase
	}

	dnsZones := []DNSZone{
		{
			Id:     1,
			Domain: "example.com",
			Records: []DNSRecord{
				{Id: 1, Type: 0, Name: "example.com", Value: "1.2.3.4", TTL: 300},            // A record
				{Id: 2, Type: 2, Name: "WWW.example.COM", Value: "example.com", TTL: 3600},   // CNAME record with mixed case
				{Id: 3, Type: 15, Name: "example.com", Value: "mail.example.com", TTL: 3600}, // MX record (should be filtered)
			},
		},
	}

	// Create hostname map using side effect free function
	hostnameMap := createHostnameMap(hostnames)

	// Filter records using side effect free function
	result := filterMatchingDNSRecords(dnsZones, hostnameMap)

	expected := []DNSRecordFormatted{
		{Name: "example.com", Type: "A", Value: "1.2.3.4", TTL: 300},
		{Name: "WWW.example.COM", Type: "CNAME", Value: "example.com", TTL: 3600},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Integration test failed. Got %v, want %v", result, expected)
	}
}
