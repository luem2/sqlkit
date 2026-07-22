package docs

import (
	"regexp"
	"strings"
	"unicode"
)

func splitSQLLineComment(line string) (string, string) {
	idx := strings.Index(line, "--")
	if idx < 0 {
		return line, ""
	}
	return line[:idx], line[idx+2:]
}

func stripSQLComments(content string) string {
	lineComment := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(content, "")
	blockComment := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(lineComment, "")
	return blockComment
}

func splitTopLevelComma(s string) []string {
	var parts []string
	start := 0
	depth := 0
	inString := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func findTopLevelKeyword(s string, keyword string) int {
	depth := 0
	inString := false
	for i := 0; i < len(s)-len(keyword)+1; i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && isKeywordAt(s, i, keyword) {
				return i
			}
		}
	}
	return -1
}

func findTopLevelFrom(s string, start int) int {
	depth := 0
	inString := false
	for i := start; i < len(s)-4; i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && isKeywordAt(s, i, "FROM") {
				return i
			}
		}
	}
	return -1
}

func isKeywordAt(s string, pos int, keyword string) bool {
	if pos+len(keyword) > len(s) || !strings.EqualFold(s[pos:pos+len(keyword)], keyword) {
		return false
	}
	beforeOK := pos == 0 || !isIdentifierRune(rune(s[pos-1]))
	after := pos + len(keyword)
	afterOK := after >= len(s) || !isIdentifierRune(rune(s[after]))
	return beforeOK && afterOK
}

func keywordCount(s string, keyword string) int {
	count := 0
	for i := 0; i < len(s)-len(keyword)+1; i++ {
		if isKeywordAt(s, i, keyword) {
			count++
			i += len(keyword) - 1
		}
	}
	return count
}

func isIdentifierRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '@'
}

func lastTopLevelWhitespace(s string) int {
	depth := 0
	inString := false
	last := -1
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && unicode.IsSpace(rune(ch)) {
				last = i
			}
		}
	}
	return last
}

func firstTableAfterFrom(afterFrom string) string {
	afterFrom = strings.TrimSpace(afterFrom)
	match := regexp.MustCompile(`(?is)^(#\w+|\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*(?:\s*\.\s*(?:\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*))?)`).FindStringSubmatch(afterFrom)
	if len(match) < 2 {
		return ""
	}
	return strings.Join(strings.Fields(match[1]), "")
}

func trimIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	}
	return value
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.ReplaceAll(value, "\n", " ")
}
