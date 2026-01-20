package logger

import (
	"regexp"
)

// sanitizeSQL sanitizes SQL statements
// Replace sensitive information (passwords, phone numbers, ID numbers, etc.) with ***
func sanitizeSQL(sql string) string {
	// Mask password field
	sql = sanitizePassword(sql)

	// mask phone number
	sql = sanitizePhone(sql)

	// Mask sensitive ID number
	sql = sanitizeIDCard(sql)

	return sql
}

// sanitizePassword sanitize password field
func sanitizePassword(sql string) string {
	// match password = 'xxx' or password='xxx'
	re := regexp.MustCompile(`(?i)(password\s*=\s*['"])([^'"]+)(['"])`)
	return re.ReplaceAllString(sql, `$1***$3`)
}

// sanitizePhone mask phone number
func sanitizePhone(sql string) string {
	// Match 11-digit mobile numbers, retain the first 3 digits and last 4 digits
	re := regexp.MustCompile(`(\d{3})\d{4}(\d{4})`)
	return re.ReplaceAllString(sql, `$1****$2`)
}

// sanitizeIDCard mask ID number
func sanitizeIDCard(sql string) string {
	// Match 18-digit ID numbers, retain the first 6 digits and last 4 digits
	re := regexp.MustCompile(`(\d{6})\d{8}(\d{4})`)
	return re.ReplaceAllString(sql, `$1********$2`)
}

