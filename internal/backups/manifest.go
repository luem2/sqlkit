package backups

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Manifest struct {
	Environment       string    `json:"environment"`
	Database          string    `json:"database"`
	Type              string    `json:"type"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	File              string    `json:"file"`
	ServerFile        string    `json:"server_file"`
	ManifestFile      string    `json:"manifest_file"`
	SizeBytes         int64     `json:"size_bytes"`
	SHA256            string    `json:"sha256,omitempty"`
	SQLChecksum       bool      `json:"sql_checksum"`
	S3URI             string    `json:"s3_uri,omitempty"`
	S3ManifestURI     string    `json:"s3_manifest_uri,omitempty"`
	S3Uploaded        bool      `json:"s3_uploaded"`
	Status            string    `json:"status"`
	Error             string    `json:"error,omitempty"`
	FirstLSN          string    `json:"first_lsn,omitempty"`
	LastLSN           string    `json:"last_lsn,omitempty"`
	CheckpointLSN     string    `json:"checkpoint_lsn,omitempty"`
	DatabaseBackupLSN string    `json:"database_backup_lsn,omitempty"`
	BackupStartDate   string    `json:"backup_start_date,omitempty"`
	BackupFinishDate  string    `json:"backup_finish_date,omitempty"`
}

type BackupMetadata struct {
	FirstLSN          string
	LastLSN           string
	CheckpointLSN     string
	DatabaseBackupLSN string
	BackupStartDate   string
	BackupFinishDate  string
}

func WriteManifest(manifest *Manifest) error {
	if err := EnsureParent(manifest.ManifestFile); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(manifest.ManifestFile, data, 0o640)
}

func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if strings.TrimSpace(manifest.ManifestFile) == "" {
		manifest.ManifestFile = path
	}
	return &manifest, nil
}

func LoadManifests(root string) ([]Manifest, error) {
	var manifests []Manifest

	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return manifests, nil
		}
		return nil, err
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		manifest, err := ReadManifest(path)
		if err != nil {
			return fmt.Errorf("read manifest %s: %w", path, err)
		}
		manifests = append(manifests, *manifest)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].FinishedAt.Before(manifests[j].FinishedAt)
	})
	return manifests, nil
}

func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func ApplyMetadata(manifest *Manifest, metadata BackupMetadata) {
	manifest.FirstLSN = metadata.FirstLSN
	manifest.LastLSN = metadata.LastLSN
	manifest.CheckpointLSN = metadata.CheckpointLSN
	manifest.DatabaseBackupLSN = metadata.DatabaseBackupLSN
	manifest.BackupStartDate = metadata.BackupStartDate
	manifest.BackupFinishDate = metadata.BackupFinishDate
}
