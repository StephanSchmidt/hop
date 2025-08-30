package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	// #nosec G304 - filePath comes from filepath.Walk which validates the path
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

func listRemoteFiles(ctx context.Context, storageZone *StorageZone, remotePath string) ([]RemoteFileInfo, error) {
	url := fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", storageZone.Name, strings.TrimPrefix(remotePath, "/"))
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

func uploadFileToStorage(ctx context.Context, storageZone *StorageZone, localPath, remotePath string) error {
	// Read the file
	// #nosec G304 - localPath comes from filepath.Walk which validates the path
	fileContent, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", localPath, err)
	}

	// Construct the storage URL
	url := fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", storageZone.Name, strings.TrimPrefix(remotePath, "/"))

	// Create PUT request
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(fileContent))
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

// buildLocalFileMap builds a complete map of local files with checksums
func buildLocalFileMap(localDir string) (map[string]LocalFileInfo, error) {
	localFileMap := make(map[string]LocalFileInfo)

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		// Calculate checksum
		checksum, err := calculateFileChecksum(path)
		if err != nil {
			fmt.Printf("⚠ Warning: Could not calculate checksum for %s: %v\n", relPath, err)
			checksum = ""
		}

		localFileMap[strings.ReplaceAll(relPath, "\\", "/")] = LocalFileInfo{
			Path:     path,
			Size:     info.Size(),
			Checksum: checksum,
			RelPath:  strings.ReplaceAll(relPath, "\\", "/"),
		}

		return nil
	})

	return localFileMap, err
}

// remoteFileStreamer streams remote files to the skip checker
func remoteFileStreamer(ctx context.Context, storageZone *StorageZone, remoteDir string, remoteFiles chan<- RemoteFileInfo) {
	defer close(remoteFiles)

	fmt.Println("Streaming remote file list...")

	var streamFiles func(string) error
	streamFiles = func(currentPath string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		files, err := listRemoteFiles(ctx, storageZone, currentPath)
		if err != nil {
			fmt.Printf("⚠ Warning: Could not list remote files in %s: %v\n", currentPath, err)
			return nil // Continue with other directories
		}

		for _, file := range files {
			if file.IsDirectory {
				// Recursively stream subdirectories
				subPath := filepath.Join(currentPath, file.Name)
				subPath = strings.ReplaceAll(subPath, "\\", "/")
				if err := streamFiles(subPath); err != nil {
					return err
				}
			} else {
				// Stream the file
				relPath := file.Name
				if currentPath != "" && currentPath != "/" {
					relPath = filepath.Join(strings.TrimPrefix(currentPath, remoteDir), file.Name)
					relPath = strings.ReplaceAll(relPath, "\\", "/")
				}
				file.Path = relPath

				remoteFiles <- file
			}
		}
		return nil
	}

	if err := streamFiles(remoteDir); err != nil {
		fmt.Printf("⚠ Warning: Error streaming remote files: %v\n", err)
	}
}

// skipChecker processes streamed remote files and manages local file states
func skipChecker(localStates map[string]*LocalFileState, remoteFiles <-chan RemoteFileInfo, uploadTasks chan<- FileUploadTask, remoteDir string, results chan<- FileUploadStatus) {
	defer close(uploadTasks)

	remoteCount := 0
	remoteOnlyCount := 0

	// Process streamed remote files
	for remoteFile := range remoteFiles {
		remoteCount++

		// Look up corresponding local file
		localState, exists := localStates[remoteFile.Path]
		if !exists {
			// Remote file doesn't exist locally - ignore it
			remoteOnlyCount++
			continue
		}

		// Mark as checked
		localState.Checked = true

		// Check if we should skip this file
		if skip, reason := shouldSkipUpload(localState.File, remoteFile); skip {
			localState.Skip = true
			localState.Reason = reason

			results <- FileUploadStatus{
				Path:    localState.File.Path,
				Success: true,
				Skipped: true,
				Reason:  reason,
			}
		} else {
			// Need to upload this file
			remotePath := filepath.Join(remoteDir, localState.File.RelPath)
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			uploadTasks <- FileUploadTask{
				LocalFile:  localState.File,
				RemotePath: remotePath,
			}
		}
	}

	fmt.Printf("Processed %d remote files for comparison (%d remote-only files ignored)\n", remoteCount, remoteOnlyCount)

	// Process any unchecked local files (they are new files)
	for _, localState := range localStates {
		if !localState.Checked && !localState.Skip {
			// This is a new local file - needs uploading
			remotePath := filepath.Join(remoteDir, localState.File.RelPath)
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")

			uploadTasks <- FileUploadTask{
				LocalFile:  localState.File,
				RemotePath: remotePath,
			}
		}
	}
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

// LocalFileState tracks the state of local files during processing
type LocalFileState struct {
	File    LocalFileInfo
	Checked bool
	Skip    bool
	Reason  string
}

func uploadDirectoryOptimized(ctx context.Context, storageZone *StorageZone, localDir, remoteDir string) []FileUploadStatus {
	fmt.Println("Starting streaming concurrent file upload...")

	// Build complete local file list with checksums first
	fmt.Println("Building local file list with checksums...")
	localFileMap, err := buildLocalFileMap(localDir)
	if err != nil {
		return []FileUploadStatus{{
			Path:    localDir,
			Success: false,
			Error:   fmt.Errorf("failed to build local file list: %v", err),
		}}
	}

	fmt.Printf("Found %d local files\n", len(localFileMap))

	// Initialize local file states
	localStates := make(map[string]*LocalFileState)
	for relPath, localFile := range localFileMap {
		localStates[relPath] = &LocalFileState{
			File:    localFile,
			Checked: false,
			Skip:    false,
			Reason:  "",
		}
	}

	// Channels for communication between goroutines
	remoteFiles := make(chan RemoteFileInfo, 100)
	uploadTasks := make(chan FileUploadTask, 10)
	results := make(chan FileUploadStatus, 100)

	// Start remote file streamer
	go remoteFileStreamer(ctx, storageZone, remoteDir, remoteFiles)

	// Start skip checker that processes streamed remote files
	go skipChecker(localStates, remoteFiles, uploadTasks, remoteDir, results)

	// Start 8 parallel uploader goroutines
	const numWorkers = 8
	var uploaderWG sync.WaitGroup
	uploaderWG.Add(numWorkers)

	for range numWorkers {
		go func() {
			defer uploaderWG.Done()
			uploader(ctx, storageZone, uploadTasks, results)
		}()
	}

	// Close results channel when all uploaders are done
	go func() {
		uploaderWG.Wait()
		close(results)
	}()

	// Collect results
	var allResults []FileUploadStatus
	skipped := 0
	uploaded := 0
	failed := 0

	// We need to know when processing is done
	done := make(chan bool, 1)

	go func() {
		for result := range results {
			allResults = append(allResults, result)

			if result.Success {
				if result.Skipped {
					fmt.Printf("⏭ Skipped: %s (%s)\n", filepath.Base(result.Path), result.Reason)
					skipped++
				} else {
					fmt.Printf("✓ Uploaded: %s\n", filepath.Base(result.Path))
					uploaded++
				}
			} else {
				fmt.Printf("✗ Failed: %s (%v)\n", filepath.Base(result.Path), result.Error)
				failed++
			}
		}
		done <- true
	}()

	<-done // Wait for everything to complete

	uploadedWord := "file"
	if uploaded != 1 {
		uploadedWord = "files"
	}
	skippedWord := "file"
	if skipped != 1 {
		skippedWord = "files"
	}
	failedWord := "file"
	if failed != 1 {
		failedWord = "files"
	}
	fmt.Printf("\n%d %s uploaded, %d %s skipped, %d %s failed\n",
		uploaded, uploadedWord, skipped, skippedWord, failed, failedWord)
	return allResults
}

// uploader handles the actual file uploads
func uploader(ctx context.Context, storageZone *StorageZone, uploadTasks <-chan FileUploadTask, results chan<- FileUploadStatus) {
	for {
		select {
		case task, ok := <-uploadTasks:
			if !ok {
				return
			}
			err := uploadFileToStorage(ctx, storageZone, task.LocalFile.Path, task.RemotePath)

			results <- FileUploadStatus{
				Path:    task.LocalFile.Path,
				Success: err == nil,
				Error:   err,
			}
		case <-ctx.Done():
			return
		}
	}
}
