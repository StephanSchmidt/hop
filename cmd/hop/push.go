package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileUploadStatus struct {
	Path    string
	Success bool
	Error   error
	Skipped bool
	Reason  string
}

type RemoteFileInfo struct {
	Name         string    `json:"ObjectName"`
	IsDirectory  bool      `json:"IsDirectory"`
	Size         int64     `json:"Length"`
	LastModified BunnyTime `json:"LastChanged"`
	Checksum     string    `json:"Checksum"`
	Path         string
}

type LocalFileInfo struct {
	Path     string
	Size     int64
	Checksum string
	RelPath  string
}

func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("error calculating hash: %v", err)
	}

	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

func listRemoteFiles(storageZone *StorageZone, remotePath string) ([]RemoteFileInfo, error) {
	url := fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", storageZone.Name, strings.TrimPrefix(remotePath, "/"))
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("AccessKey", storageZone.Password)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error listing files: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Directory doesn't exist, return empty list
		return []RemoteFileInfo{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list files failed with status %s: %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var remoteFiles []RemoteFileInfo
	if err := json.Unmarshal(body, &remoteFiles); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	// Set the path for each file
	for i := range remoteFiles {
		if !remoteFiles[i].IsDirectory {
			remoteFiles[i].Path = filepath.Join(remotePath, remoteFiles[i].Name)
			remoteFiles[i].Path = strings.ReplaceAll(remoteFiles[i].Path, "\\", "/")
		}
	}

	return remoteFiles, nil
}

func buildRemoteFileMap(storageZone *StorageZone, remoteDir string) (map[string]RemoteFileInfo, error) {
	fileMap := make(map[string]RemoteFileInfo)

	var collectFiles func(string) error
	collectFiles = func(currentPath string) error {
		files, err := listRemoteFiles(storageZone, currentPath)
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDirectory {
				// Recursively list subdirectories
				subPath := filepath.Join(currentPath, file.Name)
				subPath = strings.ReplaceAll(subPath, "\\", "/")
				if err := collectFiles(subPath); err != nil {
					return err
				}
			} else {
				// Add file to map with relative path as key
				relPath := file.Name
				if currentPath != "" && currentPath != "/" {
					relPath = filepath.Join(strings.TrimPrefix(currentPath, remoteDir), file.Name)
					relPath = strings.ReplaceAll(relPath, "\\", "/")
				}
				fileMap[relPath] = file
			}
		}
		return nil
	}

	err := collectFiles(remoteDir)
	return fileMap, err
}

func shouldSkipUpload(localFile LocalFileInfo, remoteFile RemoteFileInfo) (bool, string) {
	// Compare size first (quick check)
	if localFile.Size != remoteFile.Size {
		return false, ""
	}

	// If checksums are available, compare them
	if remoteFile.Checksum != "" && localFile.Checksum != "" {
		if localFile.Checksum == remoteFile.Checksum {
			return true, "checksum match"
		}
		return false, ""
	}

	// Fallback to size comparison only
	if localFile.Size == remoteFile.Size {
		return true, "size match (no checksum)"
	}

	return false, ""
}

func uploadFileToStorage(storageZone *StorageZone, localPath, remotePath string) error {
	// Read the file
	fileContent, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", localPath, err)
	}

	// Construct the storage URL
	url := fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", storageZone.Name, strings.TrimPrefix(remotePath, "/"))

	// Create PUT request
	req, err := http.NewRequest("PUT", url, bytes.NewReader(fileContent))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("AccessKey", storageZone.Password)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error uploading file: %v", err)
	}
	if resp == nil {
		return fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %s: %s", resp.Status, string(body))
	}

	return nil
}

// FileProcessTask represents a file that needs processing
type FileProcessTask struct {
	Path    string
	RelPath string
	Size    int64
}

// FileUploadTask represents a file ready for upload
type FileUploadTask struct {
	LocalFile  LocalFileInfo
	RemotePath string
}

func uploadDirectoryOptimized(storageZone *StorageZone, localDir, remoteDir string) []FileUploadStatus {
	fmt.Println("Starting concurrent file upload...")
	
	// Get remote file list first
	fmt.Println("Fetching remote file list...")
	remoteFileMap, err := buildRemoteFileMap(storageZone, remoteDir)
	if err != nil {
		fmt.Printf("⚠ Warning: Could not fetch remote file list: %v\n", err)
		fmt.Println("Proceeding with upload without optimization...")
		remoteFileMap = make(map[string]RemoteFileInfo)
	} else {
		fmt.Printf("Found %d remote files\n", len(remoteFileMap))
	}

	// Channels for communication between goroutines
	fileProcessTasks := make(chan FileProcessTask, 100)
	uploadTasks := make(chan FileUploadTask, 10)
	results := make(chan FileUploadStatus, 100)
	
	// Start file processor goroutine
	go fileProcessor(fileProcessTasks, uploadTasks, remoteFileMap, remoteDir, results)
	
	// Start uploader goroutine
	go uploader(storageZone, uploadTasks, results)
	
	// Walk the directory and send files to processor
	go func() {
		defer close(fileProcessTasks)
		
		filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				results <- FileUploadStatus{
					Path:    path,
					Success: false,
					Error:   err,
				}
				return nil
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Calculate relative path
			relPath, err := filepath.Rel(localDir, path)
			if err != nil {
				results <- FileUploadStatus{
					Path:    path,
					Success: false,
					Error:   err,
				}
				return nil
			}

			// Send to processor
			fileProcessTasks <- FileProcessTask{
				Path:    path,
				RelPath: strings.ReplaceAll(relPath, "\\", "/"),
				Size:    info.Size(),
			}
			
			return nil
		})
	}()
	
	// Collect results
	var allResults []FileUploadStatus
	skipped := 0
	uploaded := 0
	failed := 0
	
	// We need to know when both goroutines are done
	done := make(chan bool, 2)
	
	go func() {
		for result := range results {
			allResults = append(allResults, result)
			
			if result.Success {
				if result.Skipped {
					fmt.Printf("⏭ Skipped: %s (%s)\n", result.Path, result.Reason)
					skipped++
				} else {
					fmt.Printf("✓ Uploaded: %s\n", result.Path)
					uploaded++
				}
			} else {
				fmt.Printf("✗ Failed: %s (%v)\n", result.Path, result.Error)
				failed++
			}
		}
		done <- true
	}()
	
	// Wait for processing and uploading to complete
	go func() {
		<-done // Wait for result collection to finish
		done <- true
	}()
	
	<-done // Wait for everything to complete
	
	fmt.Printf("\n%d uploaded, %d skipped, %d failed\n", uploaded, skipped, failed)
	return allResults
}

// fileProcessor processes files, calculates checksums, and determines what needs uploading
func fileProcessor(tasks <-chan FileProcessTask, uploadTasks chan<- FileUploadTask, remoteFileMap map[string]RemoteFileInfo, remoteDir string, results chan<- FileUploadStatus) {
	defer close(uploadTasks)
	
	for task := range tasks {
		// Calculate checksum
		checksum, err := calculateFileChecksum(task.Path)
		if err != nil {
			fmt.Printf("⚠ Warning: Could not calculate checksum for %s: %v\n", task.RelPath, err)
			checksum = ""
		}
		
		localFile := LocalFileInfo{
			Path:     task.Path,
			Size:     task.Size,
			Checksum: checksum,
			RelPath:  task.RelPath,
		}
		
		// Check if we should skip this file
		remoteFile, exists := remoteFileMap[task.RelPath]
		if exists {
			if skip, reason := shouldSkipUpload(localFile, remoteFile); skip {
				results <- FileUploadStatus{
					Path:    task.Path,
					Success: true,
					Skipped: true,
					Reason:  reason,
				}
				continue
			}
		}
		
		// Convert to forward slashes for URL paths
		remotePath := filepath.Join(remoteDir, task.RelPath)
		remotePath = strings.ReplaceAll(remotePath, "\\", "/")
		
		// Send to uploader
		uploadTasks <- FileUploadTask{
			LocalFile:  localFile,
			RemotePath: remotePath,
		}
	}
}

// uploader handles the actual file uploads
func uploader(storageZone *StorageZone, uploadTasks <-chan FileUploadTask, results chan<- FileUploadStatus) {
	defer close(results)
	
	for task := range uploadTasks {
		err := uploadFileToStorage(storageZone, task.LocalFile.Path, task.RemotePath)
		
		results <- FileUploadStatus{
			Path:    task.LocalFile.Path,
			Success: err == nil,
			Error:   err,
		}
	}
}
