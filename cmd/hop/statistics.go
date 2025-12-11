package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StatisticsResponse represents the response from the Bunny.net statistics API
type StatisticsResponse struct {
	TotalBandwidthUsed      int64                 `json:"TotalBandwidthUsed"`
	TotalRequestsServed     int64                 `json:"TotalRequestsServed"`
	CacheHitRate            float64               `json:"CacheHitRate"`
	BandwidthUsedChart      []ChartData           `json:"BandwidthUsedChart"`
	RequestsServedChart     []ChartData           `json:"RequestsServedChart"`
	PullRequestsPulledChart []ChartData           `json:"PullRequestsPulledChart"`
	UserBalanceHistoryChart []ChartData           `json:"UserBalanceHistoryChart"`
	GeoTrafficDistribution  []GeoDistributionData `json:"GeoTrafficDistribution"`
	Error                   *string               `json:"Error"`
	ErrorKey                *string               `json:"ErrorKey"`
}

// ChartData represents data points in various charts
type ChartData struct {
	Timestamp BunnyTime `json:"Timestamp"`
	Value     int64     `json:"Value"`
}

// GeoDistributionData represents geographic traffic distribution
type GeoDistributionData struct {
	CountryCode    string `json:"CountryCode"`
	CountryName    string `json:"CountryName"`
	BandwidthUsed  int64  `json:"BandwidthUsed"`
	RequestsServed int64  `json:"RequestsServed"`
}

// getStatistics fetches statistics from the Bunny.net API
func getStatistics(ctx context.Context, apiKey string, params StatisticsParams) (*StatisticsResponse, error) {
	// Build the URL with query parameters
	baseURL := "https://api.bunny.net/statistics"
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	// Add query parameters
	query := u.Query()
	if !params.DateFrom.IsZero() {
		query.Set("dateFrom", params.DateFrom.Format("1-2-2006"))
	}
	if !params.DateTo.IsZero() {
		query.Set("dateTo", params.DateTo.Format("1-2-2006"))
	}
	if params.PullZoneID > 0 {
		query.Set("pullZone", fmt.Sprintf("%d", params.PullZoneID))
	}
	if params.ServerZoneID > 0 {
		query.Set("serverZoneId", fmt.Sprintf("%d", params.ServerZoneID))
	}
	if params.Hourly {
		query.Set("hourly", "true")
	}
	if params.LoadBandwidthUsed {
		query.Set("loadBandwidthUsed", "true")
	}
	if params.LoadRequestsServed {
		query.Set("loadRequestsServed", "true")
	}
	if params.LoadGeographicTrafficDistribution {
		query.Set("loadGeographicTrafficDistribution", "true")
	}
	if params.LoadUserBalanceHistory {
		query.Set("loadUserBalanceHistory", "true")
	}

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %s: %s", resp.Status, string(body))
	}

	var stats StatisticsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	if stats.Error != nil {
		return nil, fmt.Errorf("API returned error: %s", *stats.Error)
	}

	return &stats, nil
}

// StatisticsParams holds parameters for the statistics API call
type StatisticsParams struct {
	DateFrom                          time.Time
	DateTo                            time.Time
	PullZoneID                        int64
	ServerZoneID                      int64
	Hourly                            bool
	LoadBandwidthUsed                 bool
	LoadRequestsServed                bool
	LoadGeographicTrafficDistribution bool
	LoadUserBalanceHistory            bool
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatNumber formats numbers with comma separators
func formatNumber(num int64) string {
	str := fmt.Sprintf("%d", num)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}

// displayStatisticsSummary displays a summary of the statistics
func displayStatisticsSummary(stats *StatisticsResponse) {
	fmt.Printf("SUMMARY\n")
	fmt.Println(strings.Repeat("-", 40))

	fmt.Printf("Total Bandwidth Used: %s\n", formatBytes(stats.TotalBandwidthUsed))
	fmt.Printf("Total Requests Served: %s\n", formatNumber(stats.TotalRequestsServed))
	fmt.Printf("Cache Hit Rate: %.1f%%\n", stats.CacheHitRate)

	if len(stats.GeoTrafficDistribution) > 0 {
		fmt.Printf("\nTOP COUNTRIES BY BANDWIDTH\n")
		fmt.Println(strings.Repeat("-", 40))

		// Show top 5 countries
		count := len(stats.GeoTrafficDistribution)
		if count > 5 {
			count = 5
		}

		for i := 0; i < count; i++ {
			geo := stats.GeoTrafficDistribution[i]
			fmt.Printf("%-3s %-20s %s (%s requests)\n",
				geo.CountryCode,
				geo.CountryName,
				formatBytes(geo.BandwidthUsed),
				formatNumber(geo.RequestsServed))
		}
	}
}

// displayBandwidthChart displays bandwidth usage over time
func displayBandwidthChart(data []ChartData, title string) {
	if len(data) == 0 {
		fmt.Printf("No %s data available\n", title)
		return
	}

	fmt.Printf("\n%s\n", strings.ToUpper(title))
	fmt.Println(strings.Repeat("-", 40))

	for _, point := range data {
		timeStr := point.Timestamp.Time.Format("2006-01-02 15:04")
		fmt.Printf("%s: %s\n", timeStr, formatBytes(point.Value))
	}
}

// displayRequestsChart displays request counts over time
func displayRequestsChart(data []ChartData, title string) {
	if len(data) == 0 {
		fmt.Printf("No %s data available\n", title)
		return
	}

	fmt.Printf("\n%s\n", strings.ToUpper(title))
	fmt.Println(strings.Repeat("-", 40))

	for _, point := range data {
		timeStr := point.Timestamp.Time.Format("2006-01-02 15:04")
		fmt.Printf("%s: %s requests\n", timeStr, formatNumber(point.Value))
	}
}

// displayGeographicDistribution displays geographic traffic distribution
func displayGeographicDistribution(data []GeoDistributionData) {
	if len(data) == 0 {
		fmt.Printf("No geographic distribution data available\n")
		return
	}

	fmt.Printf("\nGEOGRAPHIC DISTRIBUTION\n")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-3s %-20s %-15s %-15s\n", "Code", "Country", "Bandwidth", "Requests")
	fmt.Println(strings.Repeat("-", 70))

	for _, geo := range data {
		fmt.Printf("%-3s %-20s %-15s %-15s\n",
			geo.CountryCode,
			geo.CountryName,
			formatBytes(geo.BandwidthUsed),
			formatNumber(geo.RequestsServed))
	}
}
