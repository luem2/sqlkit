package backups

import (
	"testing"
	"time"
)

func TestEvaluateHealthDetectsStaleAndMissingBackups(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	policy := &Policy{
		Databases: []string{"P_BD_SISTEMA"},
		Alerts: AlertThreshold{
			FullBackupMaxAgeHours:  36,
			DiffBackupMaxAgeHours:  36,
			LogBackupMaxAgeMinutes: 30,
		},
	}
	manifests := []Manifest{
		okManifest("P_BD_SISTEMA", TypeFull, "full.bak", now.Add(-24*time.Hour)),
		okManifest("P_BD_SISTEMA", TypeLog, "log.trn", now.Add(-45*time.Minute)),
	}

	health := EvaluateHealth(policy, manifests, now)
	if health.OK {
		t.Fatal("expected unhealthy result")
	}
	if health.Databases[0].Backups[TypeDiff].Status != "missing" {
		t.Fatalf("unexpected diff status: %#v", health.Databases[0].Backups[TypeDiff])
	}
	if health.Databases[0].Backups[TypeLog].Status != "stale" {
		t.Fatalf("unexpected log status: %#v", health.Databases[0].Backups[TypeLog])
	}
}
