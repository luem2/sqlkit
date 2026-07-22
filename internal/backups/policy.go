package backups

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	TypeFull = "full"
	TypeDiff = "diff"
	TypeLog  = "log"
)

var validTypes = map[string]struct{}{
	TypeFull: {},
	TypeDiff: {},
	TypeLog:  {},
}

type Policy struct {
	Environment   string         `toml:"environment"`
	Enabled       bool           `toml:"enabled"`
	LocalRoot     string         `toml:"local_root"`
	SQLServerRoot string         `toml:"sqlserver_root"`
	Container     string         `toml:"container"`
	RemoteCopy    RemoteCopy     `toml:"remote_copy"`
	S3Prefix      string         `toml:"s3_prefix"`
	Databases     []string       `toml:"databases"`
	Schedule      Schedule       `toml:"schedule"`
	Retention     Retention      `toml:"retention"`
	RestoreDrill  RestoreDrill   `toml:"restore_drill"`
	Alerts        AlertThreshold `toml:"alerts"`
}

type RemoteCopy struct {
	Enabled      bool   `toml:"enabled"`
	Host         string `toml:"host"`
	User         string `toml:"user"`
	Port         int    `toml:"port"`
	IdentityFile string `toml:"identity_file"`
	KeepRemote   bool   `toml:"keep_remote"`
}

type Schedule struct {
	Full string `toml:"full"`
	Diff string `toml:"diff"`
	Log  string `toml:"log"`
}

type Retention struct {
	FullDays int `toml:"full_days"`
	DiffDays int `toml:"diff_days"`
	LogDays  int `toml:"log_days"`
}

type RestoreDrill struct {
	Enabled        bool   `toml:"enabled"`
	TargetEnv      string `toml:"target_env"`
	DatabaseSuffix string `toml:"database_suffix"`
	CheckDB        bool   `toml:"checkdb"`
}

type AlertThreshold struct {
	LogBackupMaxAgeMinutes int `toml:"log_backup_max_age_minutes"`
	FullBackupMaxAgeHours  int `toml:"full_backup_max_age_hours"`
	DiffBackupMaxAgeHours  int `toml:"diff_backup_max_age_hours"`
	DiskUsageMaxPercent    int `toml:"disk_usage_max_percent"`
}

func LoadPolicy(path string) (*Policy, error) {
	policy, err := DecodePolicy(path)
	if err != nil {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return policy, nil
}

func DecodePolicy(path string) (*Policy, error) {
	var policy Policy
	if _, err := toml.DecodeFile(path, &policy); err != nil {
		return nil, err
	}
	policy.applyDefaults()
	return &policy, nil
}

func (p *Policy) applyDefaults() {
	if strings.TrimSpace(p.LocalRoot) == "" {
		p.LocalRoot = "backups"
	}
	if strings.TrimSpace(p.SQLServerRoot) == "" {
		p.SQLServerRoot = p.LocalRoot
	}
	if p.RemoteCopy.Enabled && p.RemoteCopy.Port == 0 {
		p.RemoteCopy.Port = 22
	}
	if p.Retention.FullDays == 0 {
		p.Retention.FullDays = 60
	}
	if p.Retention.DiffDays == 0 {
		p.Retention.DiffDays = 15
	}
	if p.Retention.LogDays == 0 {
		p.Retention.LogDays = 15
	}
	if strings.TrimSpace(p.RestoreDrill.DatabaseSuffix) == "" {
		p.RestoreDrill.DatabaseSuffix = "_RESTORE_DRILL"
	}
	if p.Alerts.LogBackupMaxAgeMinutes == 0 {
		p.Alerts.LogBackupMaxAgeMinutes = 30
	}
	if p.Alerts.FullBackupMaxAgeHours == 0 {
		p.Alerts.FullBackupMaxAgeHours = 36
	}
	if p.Alerts.DiffBackupMaxAgeHours == 0 {
		p.Alerts.DiffBackupMaxAgeHours = 36
	}
	if p.Alerts.DiskUsageMaxPercent == 0 {
		p.Alerts.DiskUsageMaxPercent = 85
	}
}

func (p *Policy) Validate() error {
	if strings.TrimSpace(p.Environment) == "" {
		return fmt.Errorf("policy environment is required")
	}
	if len(p.Databases) == 0 {
		return fmt.Errorf("policy databases are required")
	}
	seen := map[string]struct{}{}
	for _, database := range p.Databases {
		database = strings.TrimSpace(database)
		if database == "" {
			return fmt.Errorf("policy databases cannot contain empty values")
		}
		key := strings.ToLower(database)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate database in policy: %s", database)
		}
		seen[key] = struct{}{}
	}
	if p.Retention.FullDays <= 0 || p.Retention.DiffDays <= 0 || p.Retention.LogDays <= 0 {
		return fmt.Errorf("retention days must be positive")
	}
	if p.RemoteCopy.Enabled {
		if strings.TrimSpace(p.Container) != "" {
			return fmt.Errorf("backup policy cannot use both container and remote_copy")
		}
		if strings.TrimSpace(p.RemoteCopy.Host) == "" {
			return fmt.Errorf("remote_copy.host is required when remote_copy is enabled")
		}
		if strings.TrimSpace(p.RemoteCopy.User) == "" {
			return fmt.Errorf("remote_copy.user is required when remote_copy is enabled")
		}
		if p.RemoteCopy.Port <= 0 {
			return fmt.Errorf("remote_copy.port must be positive")
		}
	}
	return nil
}

func NormalizeType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := validTypes[value]; !ok {
		return "", fmt.Errorf("backup type must be full, diff or log")
	}
	return value, nil
}

func TypeExtension(backupType string) string {
	switch backupType {
	case TypeFull:
		return ".bak"
	case TypeDiff:
		return ".diff"
	case TypeLog:
		return ".trn"
	default:
		return ".bak"
	}
}

func RetentionDays(policy *Policy, backupType string) int {
	switch backupType {
	case TypeFull:
		return policy.Retention.FullDays
	case TypeDiff:
		return policy.Retention.DiffDays
	case TypeLog:
		return policy.Retention.LogDays
	default:
		return policy.Retention.LogDays
	}
}

func BackupPath(policy *Policy, database string, backupType string, at time.Time) string {
	fileName := fmt.Sprintf("%s_%s_%s%s", safeFileName(database), strings.ToUpper(backupType), at.Format("20060102_150405"), TypeExtension(backupType))
	return DatedPath(policy.LocalRoot, policy.Environment, database, backupType, at, fileName)
}

func SQLServerBackupPath(policy *Policy, database string, backupType string, at time.Time) string {
	fileName := fmt.Sprintf("%s_%s_%s%s", safeFileName(database), strings.ToUpper(backupType), at.Format("20060102_150405"), TypeExtension(backupType))
	return DatedPath(policy.SQLServerRoot, policy.Environment, database, backupType, at, fileName)
}

func SQLServerDrillPath(policy *Policy, file string) string {
	return storagePath(policy.SQLServerRoot, policy.Environment, "_restore_drill", filepath.Base(file))
}

func ManifestPath(policy *Policy, database string, backupType string, at time.Time) string {
	fileName := fmt.Sprintf("%s_%s_%s.json", safeFileName(database), strings.ToUpper(backupType), at.Format("20060102_150405"))
	return DatedPath(policy.LocalRoot, policy.Environment, database, "manifest", at, fileName)
}

func S3URI(policy *Policy, database string, backupType string, at time.Time, file string) string {
	prefix := strings.TrimRight(strings.TrimSpace(policy.S3Prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/" + safeFileName(policy.Environment) + "/" + safeFileName(database) + "/" + safeFileName(backupType) + "/" + at.Format("2006/01/02") + "/" + filepath.Base(file)
}

func S3ManifestURI(policy *Policy, database string, backupType string, at time.Time, file string) string {
	prefix := strings.TrimRight(strings.TrimSpace(policy.S3Prefix), "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/" + safeFileName(policy.Environment) + "/" + safeFileName(database) + "/manifest/" + at.Format("2006/01/02") + "/" + filepath.Base(file)
}

func DatedPath(root string, environment string, database string, category string, at time.Time, fileName string) string {
	return storagePath(
		root,
		safeFileName(environment),
		safeFileName(database),
		safeFileName(category),
		at.Format("2006"),
		at.Format("01"),
		at.Format("02"),
		fileName,
	)
}

func EnsureParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

func safeFileName(value string) string {
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func storagePath(root string, parts ...string) string {
	separator := "/"
	if strings.Contains(root, `\`) && !strings.Contains(root, "/") {
		separator = `\`
	}
	values := []string{strings.TrimRight(strings.TrimSpace(root), `/\`)}
	for _, part := range parts {
		values = append(values, strings.Trim(strings.TrimSpace(part), `/\`))
	}
	return strings.Join(values, separator)
}
