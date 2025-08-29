package main

import (
	"testing"
)

func TestShouldSkipUpload(t *testing.T) {
	tests := []struct {
		name       string
		localFile  LocalFileInfo
		remoteFile RemoteFileInfo
		wantSkip   bool
		wantReason string
	}{
		{
			name: "different sizes should not skip",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "ABC123",
			},
			remoteFile: RemoteFileInfo{
				Size:     200,
				Checksum: "ABC123",
			},
			wantSkip:   false,
			wantReason: "",
		},
		{
			name: "same size and matching checksums should skip",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "ABC123",
			},
			remoteFile: RemoteFileInfo{
				Size:     100,
				Checksum: "ABC123",
			},
			wantSkip:   true,
			wantReason: "checksum match",
		},
		{
			name: "same size but different checksums should not skip",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "ABC123",
			},
			remoteFile: RemoteFileInfo{
				Size:     100,
				Checksum: "DEF456",
			},
			wantSkip:   false,
			wantReason: "",
		},
		{
			name: "same size with no remote checksum should skip with size match reason",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "ABC123",
			},
			remoteFile: RemoteFileInfo{
				Size:     100,
				Checksum: "",
			},
			wantSkip:   true,
			wantReason: "size match (no checksum)",
		},
		{
			name: "same size with no local checksum should skip with size match reason",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "",
			},
			remoteFile: RemoteFileInfo{
				Size:     100,
				Checksum: "DEF456",
			},
			wantSkip:   true,
			wantReason: "size match (no checksum)",
		},
		{
			name: "same size with no checksums should skip with size match reason",
			localFile: LocalFileInfo{
				Size:     100,
				Checksum: "",
			},
			remoteFile: RemoteFileInfo{
				Size:     100,
				Checksum: "",
			},
			wantSkip:   true,
			wantReason: "size match (no checksum)",
		},
		{
			name: "zero size files with matching checksums should skip",
			localFile: LocalFileInfo{
				Size:     0,
				Checksum: "EMPTY",
			},
			remoteFile: RemoteFileInfo{
				Size:     0,
				Checksum: "EMPTY",
			},
			wantSkip:   true,
			wantReason: "checksum match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSkip, gotReason := shouldSkipUpload(tt.localFile, tt.remoteFile)
			if gotSkip != tt.wantSkip {
				t.Errorf("shouldSkipUpload() skip = %v, want %v", gotSkip, tt.wantSkip)
			}
			if gotReason != tt.wantReason {
				t.Errorf("shouldSkipUpload() reason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}