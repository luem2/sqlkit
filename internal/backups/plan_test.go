package backups

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanRestoreUsesLatestFullLatestDiffAndLaterLogs(t *testing.T) {
	base := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	manifests := []Manifest{
		lsnManifest(okManifest("P_BD_SISTEMA", TypeFull, "old-full.bak", base.Add(-48*time.Hour)), "10", "20", "15", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeLog, "old-log.trn", base.Add(-47*time.Hour)), "15", "25", "", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeFull, "full.bak", base.Add(-24*time.Hour)), "30", "40", "35", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeLog, "ignored-log.trn", base.Add(-23*time.Hour)), "35", "42", "", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeDiff, "diff.diff", base.Add(-12*time.Hour)), "35", "50", "", "35"),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeLog, "one.trn", base.Add(-11*time.Hour)), "45", "60", "", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeLog, "two.trn", base.Add(-10*time.Hour)), "60", "70", "", ""),
		okManifest("OTHER_DB", TypeFull, "other.bak", base.Add(-1*time.Hour)),
	}

	plan, err := PlanRestore("P_BD_SISTEMA", manifests, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Full.File != "full.bak" {
		t.Fatalf("Full = %q, want full.bak", plan.Full.File)
	}
	if plan.Diff == nil || plan.Diff.File != "diff.diff" {
		t.Fatalf("Diff = %#v, want diff.diff", plan.Diff)
	}
	if len(plan.Logs) != 2 || plan.Logs[0].File != "one.trn" || plan.Logs[1].File != "two.trn" {
		t.Fatalf("Logs = %#v", plan.Logs)
	}
}

func TestPlanRestoreRejectsLogChainGap(t *testing.T) {
	base := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	manifests := []Manifest{
		lsnManifest(okManifest("P_BD_SISTEMA", TypeFull, "full.bak", base), "10", "20", "15", ""),
		lsnManifest(okManifest("P_BD_SISTEMA", TypeLog, "gap.trn", base.Add(time.Hour)), "30", "40", "", ""),
	}
	if _, err := PlanRestore("P_BD_SISTEMA", manifests, time.Time{}); err == nil {
		t.Fatal("expected log chain gap error")
	}
}

func TestPruneCandidatesUsesTypeRetention(t *testing.T) {
	now := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	policy := &Policy{
		S3Prefix:  "s3://bucket/sql",
		Retention: Retention{FullDays: 60, DiffDays: 15, LogDays: 15},
	}
	manifests := []Manifest{
		okManifest("P_BD_SISTEMA", TypeFull, "keep-full.bak", now.AddDate(0, 0, -59)),
		okManifest("P_BD_SISTEMA", TypeFull, "delete-full.bak", now.AddDate(0, 0, -61)),
		okManifest("P_BD_SISTEMA", TypeLog, "delete-log.trn", now.AddDate(0, 0, -16)),
	}
	manifests[0].ManifestFile = "keep-full.json"
	manifests[1].ManifestFile = "delete-full.json"
	manifests[1].S3URI = "s3://bucket/delete-full.bak"
	manifests[2].ManifestFile = "delete-log.json"

	candidates := PruneCandidates(policy, manifests, now, true)
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].Path != "delete-full.bak" || !candidates[0].DeleteS3 || !candidates[0].DeleteManifest || !candidates[0].DeleteFile {
		t.Fatalf("unexpected first candidate: %#v", candidates[0])
	}
	if candidates[1].Path != "delete-log.trn" || !candidates[1].DeleteFile {
		t.Fatalf("unexpected second candidate: %#v", candidates[1])
	}
}

func TestDeleteLocalCandidateRemovesEmptyParentsWithinRoot(t *testing.T) {
	root := t.TempDir()
	backupFile := filepath.Join(root, "prod", "P_BD_SISTEMA", "log", "2026", "07", "01", "file.trn")
	manifestFile := filepath.Join(root, "prod", "P_BD_SISTEMA", "manifest", "2026", "07", "01", "file.json")
	if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupFile, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	candidate := PruneCandidate{
		Path:           backupFile,
		Manifest:       manifestFile,
		DeleteFile:     true,
		DeleteManifest: true,
	}
	if err := DeleteLocalCandidate(candidate, root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(backupFile); !os.IsNotExist(err) {
		t.Fatalf("backup file still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(manifestFile); !os.IsNotExist(err) {
		t.Fatalf("manifest file still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "prod")); !os.IsNotExist(err) {
		t.Fatalf("empty env directory still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root should remain: %v", err)
	}
}

func TestRemoveEmptyParentsStopsAtNonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "prod", "P_BD_SISTEMA", "log", "2026", "07", "01", "file.trn")
	keep := filepath.Join(root, "prod", "P_BD_SISTEMA", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEmptyParents(target, root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "prod", "P_BD_SISTEMA")); err != nil {
		t.Fatalf("non-empty database directory should remain: %v", err)
	}
}

func okManifest(database string, backupType string, file string, finished time.Time) Manifest {
	return Manifest{
		Database:   database,
		Type:       backupType,
		File:       file,
		FinishedAt: finished,
		Status:     "ok",
	}
}

func lsnManifest(manifest Manifest, first string, last string, checkpoint string, databaseBackup string) Manifest {
	manifest.FirstLSN = first
	manifest.LastLSN = last
	manifest.CheckpointLSN = checkpoint
	manifest.DatabaseBackupLSN = databaseBackup
	return manifest
}
