package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func getPullZoneDetails(apiKey, zoneID string) (*PullZoneDetails, error) {
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

	return &pullZone, nil
}

func getStorageZoneByPullZone(apiKey string, pullZoneID int64) (*StorageZone, error) {
	pullZoneDetails, err := getPullZoneDetails(apiKey, fmt.Sprintf("%d", pullZoneID))
	if err != nil {
		return nil, fmt.Errorf("error getting pull zone details: %v", err)
	}

	// Get all storage zones
	req, err := http.NewRequest("GET", "https://api.bunny.net/storagezone", nil)
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
