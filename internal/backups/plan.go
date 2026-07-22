package backups

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RestorePlan struct {
	Database string
	Full     Manifest
	Diff     *Manifest
	Logs     []Manifest
}

type PruneCandidate struct {
	Path           string
	S3URI          string
	S3ManifestURI  string
	Type           string
	Database       string
	Finished       time.Time
	Manifest       string
	DeleteS3       bool
	DeleteFile     bool
	DeleteManifest bool
}

func LatestByDatabaseAndType(manifests []Manifest) map[string]map[string]Manifest {
	latest := map[string]map[string]Manifest{}
	for _, manifest := range manifests {
		if manifest.Status != "ok" {
			continue
		}
		if latest[manifest.Database] == nil {
			latest[manifest.Database] = map[string]Manifest{}
		}
		current, ok := latest[manifest.Database][manifest.Type]
		if !ok || manifest.FinishedAt.After(current.FinishedAt) {
			latest[manifest.Database][manifest.Type] = manifest
		}
	}
	return latest
}

func PlanRestore(database string, manifests []Manifest, at time.Time) (RestorePlan, error) {
	var dbManifests []Manifest
	for _, manifest := range manifests {
		if manifest.Status != "ok" {
			continue
		}
		if !strings.EqualFold(manifest.Database, database) {
			continue
		}
		if !at.IsZero() && manifest.FinishedAt.After(at) {
			continue
		}
		dbManifests = append(dbManifests, manifest)
	}
	sort.Slice(dbManifests, func(i, j int) bool {
		return dbManifests[i].FinishedAt.Before(dbManifests[j].FinishedAt)
	})

	var full *Manifest
	for i := range dbManifests {
		if dbManifests[i].Type == TypeFull {
			candidate := dbManifests[i]
			full = &candidate
		}
	}
	if full == nil {
		return RestorePlan{}, fmt.Errorf("no successful full backup found for %s", database)
	}

	var diff *Manifest
	for i := range dbManifests {
		manifest := dbManifests[i]
		if manifest.Type != TypeDiff || !manifest.FinishedAt.After(full.FinishedAt) {
			continue
		}
		if full.CheckpointLSN != "" && manifest.DatabaseBackupLSN != full.CheckpointLSN {
			continue
		}
		candidate := manifest
		diff = &candidate
	}

	baseTime := full.FinishedAt
	baseLSN := full.LastLSN
	if diff != nil {
		baseTime = diff.FinishedAt
		baseLSN = diff.LastLSN
	}
	var logCandidates []Manifest
	for _, manifest := range dbManifests {
		if manifest.Type == TypeLog && manifest.FinishedAt.After(baseTime) {
			logCandidates = append(logCandidates, manifest)
		}
	}
	sort.Slice(logCandidates, func(i, j int) bool {
		return logCandidates[i].FinishedAt.Before(logCandidates[j].FinishedAt)
	})
	logs, err := continuousLogs(baseLSN, logCandidates)
	if err != nil {
		return RestorePlan{}, fmt.Errorf("%s: %w", database, err)
	}

	return RestorePlan{
		Database: database,
		Full:     *full,
		Diff:     diff,
		Logs:     logs,
	}, nil
}

func continuousLogs(baseLSN string, candidates []Manifest) ([]Manifest, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	current, ok := parseLSN(baseLSN)
	if !ok {
		return nil, fmt.Errorf("backup metadata is missing base last_lsn")
	}

	var logs []Manifest
	for _, candidate := range candidates {
		first, firstOK := parseLSN(candidate.FirstLSN)
		last, lastOK := parseLSN(candidate.LastLSN)
		if !firstOK || !lastOK {
			return nil, fmt.Errorf("log backup metadata is missing LSNs for %s", candidate.File)
		}
		if last.Cmp(current) <= 0 {
			continue
		}
		if first.Cmp(current) > 0 {
			return nil, fmt.Errorf("transaction log chain has a gap before %s", candidate.File)
		}
		logs = append(logs, candidate)
		current = last
	}
	return logs, nil
}

func parseLSN(value string) (*big.Int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	result := new(big.Int)
	parsed, ok := result.SetString(value, 10)
	return parsed, ok
}

func PruneCandidates(policy *Policy, manifests []Manifest, now time.Time, includeS3 bool) []PruneCandidate {
	var candidates []PruneCandidate
	for _, manifest := range manifests {
		if manifest.Status != "ok" {
			continue
		}
		age := now.Sub(manifest.FinishedAt)
		expired := age > time.Duration(RetentionDays(policy, manifest.Type))*24*time.Hour
		if !expired {
			continue
		}
		candidates = append(candidates, PruneCandidate{
			Path:           manifest.File,
			S3URI:          manifest.S3URI,
			S3ManifestURI:  manifest.S3ManifestURI,
			Type:           manifest.Type,
			Database:       manifest.Database,
			Finished:       manifest.FinishedAt,
			Manifest:       manifest.ManifestFile,
			DeleteS3:       includeS3 && strings.TrimSpace(manifest.S3URI) != "",
			DeleteFile:     strings.TrimSpace(manifest.File) != "",
			DeleteManifest: strings.TrimSpace(manifest.ManifestFile) != "",
		})
	}
	return candidates
}

func DeleteLocalCandidate(candidate PruneCandidate, root string) error {
	var paths []string
	if candidate.DeleteFile {
		paths = append(paths, candidate.Path)
	}
	if candidate.DeleteManifest {
		paths = append(paths, candidate.Manifest)
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := RemoveEmptyParents(path, root); err != nil {
			return err
		}
	}
	return nil
}

func RemoveEmptyParents(path string, root string) error {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absRoot = filepath.Clean(absRoot)

	dir := filepath.Dir(path)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absDir = filepath.Clean(absDir)

	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return nil
	}

	for absDir != absRoot {
		entries, err := os.ReadDir(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				// Directory was already removed. Continue with its parent.
			} else {
				return err
			}
		} else if len(entries) > 0 {
			return nil
		} else if err := os.Remove(absDir); err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			break
		}
		absDir = parent
	}
	return nil
}
