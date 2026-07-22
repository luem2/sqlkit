package ssdt

import "strings"

func ExplainSQLPackageError(output string) string {
	normalized := strings.ToLower(output)
	switch {
	case strings.Contains(normalized, "certificate chain was issued by an authority that is not trusted"),
		strings.Contains(normalized, "settings for connection encryption"),
		strings.Contains(normalized, "encrypt connection"):
		return "Check env encrypt/trust_server_certificate. For local Docker use encrypt=\"disable\". For a VM with a self-signed certificate use encrypt=\"true\" and trust_server_certificate=true."
	case strings.Contains(normalized, "login failed for user"):
		return "Check SQL user/password for the selected env."
	case strings.Contains(normalized, "could not find file"),
		strings.Contains(normalized, "no such file"),
		strings.Contains(normalized, "unable to open file"):
		return "Check dacpac, bacpac, profile and output paths."
	default:
		return ""
	}
}
