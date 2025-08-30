package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// BunnyTime handles the non-standard timestamp format from Bunny CDN API
type BunnyTime struct {
	time.Time
}

func (bt *BunnyTime) UnmarshalJSON(data []byte) error {
	// Remove quotes
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		bt.Time = time.Time{}
		return nil
	}

	// Parse the format "2025-08-29T11:10:09.594" (without timezone)
	t, err := time.Parse("2006-01-02T15:04:05.999", s)
	if err != nil {
		// Fallback to standard RFC3339 parsing
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	bt.Time = t
	return nil
}

type PullZone struct {
	Id   int64  `json:"Id"`
	Name string `json:"Name"`
}

type PullZoneDetails struct {
	Id        int64              `json:"Id"`
	Name      string             `json:"Name"`
	EdgeRules []EdgeRuleResponse `json:"EdgeRules"`
	Hostnames []Hostname         `json:"Hostnames"`
}

type Hostname struct {
	Id    int64  `json:"Id"`
	Value string `json:"Value"`
}

type StorageZone struct {
	Id       int64  `json:"Id"`
	Name     string `json:"Name"`
	Password string `json:"Password"`
}

func findPullZoneByName(ctx context.Context, apiKey, name string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.bunny.net/pullzone", nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
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

func getPullZoneDetails(ctx context.Context, apiKey, zoneID string) (*PullZoneDetails, error) {
	url := fmt.Sprintf("https://api.bunny.net/pullzone/%s", zoneID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	var pullZone PullZoneDetails
	if err := strictUnmarshal(body, &pullZone); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	return &pullZone, nil
}

func getStorageZoneByPullZone(ctx context.Context, apiKey string, pullZoneID int64) (*StorageZone, error) {
	pullZoneDetails, err := getPullZoneDetails(ctx, apiKey, fmt.Sprintf("%d", pullZoneID))
	if err != nil {
		return nil, fmt.Errorf("error getting pull zone details: %v", err)
	}

	// Get all storage zones
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.bunny.net/storagezone", nil)
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

	// Note: StorageZone is an array, can't use strictUnmarshal directly
	var storageZones []StorageZone
	if err := json.Unmarshal(body, &storageZones); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	// Find storage zone that matches the pull zone name
	for _, zone := range storageZones {
		if strings.EqualFold(zone.Name, pullZoneDetails.Name) {
			return &zone, nil
		}
	}

	return nil, fmt.Errorf("no storage zone found for pull zone '%s'", pullZoneDetails.Name)
}

// strictUnmarshal unmarshals JSON and fails if our struct has fields that don't exist in the API response
func strictUnmarshal(data []byte, v interface{}) error {
	// First, unmarshal into a map to get API fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Get expected field names from the struct
	expectedFields := getJSONFieldNames(reflect.TypeOf(v).Elem())

	// Check if our struct expects fields that don't exist in the API response
	for _, structField := range expectedFields {
		if _, exists := raw[structField]; !exists {
			return fmt.Errorf("struct expects field '%s' but it's not in the API response - remove from struct", structField)
		}
	}

	// Now unmarshal normally (this will ignore extra API fields we don't care about)
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}

	return nil
}

// getJSONFieldNames extracts JSON field names from struct tags
func getJSONFieldNames(t reflect.Type) []string {
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			// Handle json:"FieldName,omitempty" format
			name := strings.Split(jsonTag, ",")[0]
			fields = append(fields, name)
		}
	}
	return fields
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// formatBoolStatus formats a boolean as a human-readable status
func formatBoolStatus(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

// formatSSLCertificateStatus formats SSL certificate status codes
func formatSSLCertificateStatus(status int) string {
	switch status {
	case 0:
		return "Not configured"
	case 1:
		return "Pending"
	case 2:
		return "Active"
	case 3:
		return "Failed"
	case 4:
		return "Expired"
	default:
		return fmt.Sprintf("Unknown (%d)", status)
	}
}

// testSSLConnectivity tests if HTTPS works for a hostname
func testSSLConnectivity(ctx context.Context, hostname string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	url := fmt.Sprintf("https://%s/", hostname)
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Any HTTP response code means HTTPS is working (SSL handshake succeeded)
	return true
}

// testForceSSLRedirect tests if HTTP requests are redirected to HTTPS
func testForceSSLRedirect(ctx context.Context, hostname string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects, we want to check if redirect happens
			return http.ErrUseLastResponse
		},
	}

	url := fmt.Sprintf("http://%s/", hostname)
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Check if we get a redirect status code and if Location header points to HTTPS
	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		return strings.HasPrefix(strings.ToLower(location), "https://")
	}

	return false
}
