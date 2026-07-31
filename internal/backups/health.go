package backups

import (
	"fmt"
	"time"
)

type Health struct {
	OK        bool             `json:"ok"`
	CheckedAt time.Time        `json:"checked_at"`
	Problems  []string         `json:"problems"`
	Databases []DatabaseHealth `json:"databases"`
}

type DatabaseHealth struct {
	Database string                `json:"database"`
	Backups  map[string]TypeHealth `json:"backups"`
}

type TypeHealth struct {
	Status     string    `json:"status"`
	FinishedAt time.Time `json:"finished_at"`
	AgeSeconds int64     `json:"age_seconds,omitempty"`
	S3Uploaded bool      `json:"s3_uploaded"`
}

func EvaluateHealth(policy *Policy, manifests []Manifest, now time.Time) Health {
	health := Health{OK: true, CheckedAt: now, Problems: []string{}}
	latest := LatestByDatabaseAndType(manifests)

	for _, database := range policy.Databases {
		databaseHealth := DatabaseHealth{
			Database: database,
			Backups:  map[string]TypeHealth{},
		}
		for _, backupType := range []string{TypeFull, TypeDiff, TypeLog} {
			manifest, ok := latest[database][backupType]
			if !ok {
				databaseHealth.Backups[backupType] = TypeHealth{Status: "missing"}
				health.Problems = append(health.Problems, fmt.Sprintf("%s %s backup is missing", database, backupType))
				health.OK = false
				continue
			}
			age := now.Sub(manifest.FinishedAt)
			typeHealth := TypeHealth{
				Status:     "ok",
				FinishedAt: manifest.FinishedAt,
				AgeSeconds: int64(age.Seconds()),
				S3Uploaded: manifest.S3Uploaded,
			}
			if age > maxBackupAge(policy, backupType) {
				typeHealth.Status = "stale"
				health.Problems = append(health.Problems, fmt.Sprintf("%s %s backup is stale", database, backupType))
				health.OK = false
			}
			if policy.S3Prefix != "" && !manifest.S3Uploaded {
				typeHealth.Status = "s3_missing"
				health.Problems = append(health.Problems, fmt.Sprintf("%s %s backup was not uploaded to S3", database, backupType))
				health.OK = false
			}
			databaseHealth.Backups[backupType] = typeHealth
		}
		health.Databases = append(health.Databases, databaseHealth)
	}
	return health
}

func maxBackupAge(policy *Policy, backupType string) time.Duration {
	switch backupType {
	case TypeFull:
		return time.Duration(policy.Alerts.FullBackupMaxAgeHours) * time.Hour
	case TypeDiff:
		return time.Duration(policy.Alerts.DiffBackupMaxAgeHours) * time.Hour
	case TypeLog:
		return time.Duration(policy.Alerts.LogBackupMaxAgeMinutes) * time.Minute
	default:
		return 24 * time.Hour
	}
}
