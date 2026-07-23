package main

import (
	"testing"
)

func TestWWWRedirectTargets(t *testing.T) {
	tests := []struct {
		name      string
		hostnames []Hostname
		want      map[string]string
	}{
		{
			name: "www hostname yields apex target",
			hostnames: []Hostname{
				{Value: "www.example.com"},
			},
			want: map[string]string{"example.com": "www.example.com"},
		},
		{
			name: "bunny-managed hostnames skipped",
			hostnames: []Hostname{
				{Value: "www.myzone.b-cdn.net"},
			},
			want: map[string]string{},
		},
		{
			name: "non-www hostnames skipped",
			hostnames: []Hostname{
				{Value: "example.com"},
				{Value: "blog.example.com"},
			},
			want: map[string]string{},
		},
		{
			name: "mixed case normalized",
			hostnames: []Hostname{
				{Value: "WWW.Example.COM"},
			},
			want: map[string]string{"example.com": "www.example.com"},
		},
		{
			name: "multiple www hostnames",
			hostnames: []Hostname{
				{Value: "www.example.com"},
				{Value: "www.other.org"},
				{Value: "myzone.b-cdn.net"},
			},
			want: map[string]string{
				"example.com": "www.example.com",
				"other.org":   "www.other.org",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wwwRedirectTargets(tt.hostnames)
			if len(got) != len(tt.want) {
				t.Fatalf("wwwRedirectTargets() = %v, want %v", got, tt.want)
			}
			for apex, wwwHost := range tt.want {
				if got[apex] != wwwHost {
					t.Errorf("wwwRedirectTargets()[%q] = %q, want %q", apex, got[apex], wwwHost)
				}
			}
		})
	}
}

func TestIsApexRecordName(t *testing.T) {
	tests := []struct {
		name       string
		recordName string
		domain     string
		want       bool
	}{
		{name: "empty name is apex", recordName: "", domain: "example.com", want: true},
		{name: "at sign is apex", recordName: "@", domain: "example.com", want: true},
		{name: "full domain is apex", recordName: "example.com", domain: "example.com", want: true},
		{name: "full domain with trailing dot is apex", recordName: "example.com.", domain: "example.com", want: true},
		{name: "www is not apex", recordName: "www", domain: "example.com", want: false},
		{name: "subdomain is not apex", recordName: "blog.example.com", domain: "example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isApexRecordName(tt.recordName, tt.domain)
			if got != tt.want {
				t.Errorf("isApexRecordName(%q, %q) = %v, want %v", tt.recordName, tt.domain, got, tt.want)
			}
		})
	}
}

func TestNormalizeRedirectValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "trailing slash removed", value: "https://www.example.com/", want: "https://www.example.com"},
		{name: "lowercased", value: "HTTPS://WWW.Example.com", want: "https://www.example.com"},
		{name: "whitespace trimmed", value: " https://www.example.com ", want: "https://www.example.com"},
		{name: "root slash kept", value: "/", want: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRedirectValue(tt.value)
			if got != tt.want {
				t.Errorf("normalizeRedirectValue(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFindApexRedirectRecords(t *testing.T) {
	zone := DNSZone{
		Domain: "example.com",
		Records: []DNSRecord{
			{Id: 1, Type: DNSRecordTypeRedirect, Name: "", Value: "https://www.example.com/"},
			{Id: 2, Type: DNSRecordTypeRedirect, Name: "old", Value: "https://www.example.com/new/"},
			{Id: 3, Type: 0, Name: "", Value: "1.2.3.4"},
			{Id: 4, Type: 2, Name: "www", Value: "myzone.b-cdn.net"},
		},
	}

	records := findApexRedirectRecords(zone)
	if len(records) != 1 {
		t.Fatalf("findApexRedirectRecords() returned %d records, want 1", len(records))
	}
	if records[0].Id != 1 {
		t.Errorf("findApexRedirectRecords() returned record %d, want 1", records[0].Id)
	}
}

func TestCheckWWWRedirectsStructured(t *testing.T) {
	targets := map[string]string{"example.com": "www.example.com"}

	tests := []struct {
		name       string
		dnsZones   []DNSZone
		targets    map[string]string
		wantIssues int
		wantOK     int
		wantType   string
	}{
		{
			name: "correct redirect passes",
			dnsZones: []DNSZone{
				{
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: DNSRecordTypeRedirect, Name: "", Value: "https://www.example.com/"},
					},
				},
			},
			targets:    targets,
			wantIssues: 0,
			wantOK:     1,
		},
		{
			name: "redirect without trailing slash passes",
			dnsZones: []DNSZone{
				{
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: DNSRecordTypeRedirect, Name: "", Value: "https://www.example.com"},
					},
				},
			},
			targets:    targets,
			wantIssues: 0,
			wantOK:     1,
		},
		{
			name:       "missing DNS zone fails",
			dnsZones:   []DNSZone{},
			targets:    targets,
			wantIssues: 1,
			wantOK:     0,
			wantType:   "dns_www_missing_zone",
		},
		{
			name: "missing RDR record fails",
			dnsZones: []DNSZone{
				{
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: 0, Name: "", Value: "1.2.3.4"},
					},
				},
			},
			targets:    targets,
			wantIssues: 1,
			wantOK:     0,
			wantType:   "dns_www_missing_record",
		},
		{
			name: "wrong destination fails",
			dnsZones: []DNSZone{
				{
					Domain: "example.com",
					Records: []DNSRecord{
						{Id: 1, Type: DNSRecordTypeRedirect, Name: "", Value: "https://other.example.org/"},
					},
				},
			},
			targets:    targets,
			wantIssues: 1,
			wantOK:     0,
			wantType:   "dns_www_wrong_destination",
		},
		{
			name:       "no targets reports skip",
			dnsZones:   []DNSZone{},
			targets:    map[string]string{},
			wantIssues: 0,
			wantOK:     1,
			wantType:   "dns_www_skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkWWWRedirectsStructured(tt.dnsZones, tt.targets)
			if len(result.Issues) != tt.wantIssues {
				t.Fatalf("got %d issues, want %d: %+v", len(result.Issues), tt.wantIssues, result.Issues)
			}
			if len(result.Successful) != tt.wantOK {
				t.Fatalf("got %d successful, want %d: %+v", len(result.Successful), tt.wantOK, result.Successful)
			}
			if tt.wantType != "" {
				var gotType string
				if tt.wantIssues > 0 {
					gotType = result.Issues[0].Type
				} else {
					gotType = result.Successful[0].Type
				}
				if gotType != tt.wantType {
					t.Errorf("got issue type %q, want %q", gotType, tt.wantType)
				}
			}
		})
	}
}
