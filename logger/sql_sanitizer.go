package logger

import (
	"regexp"
)

// sanitizeSQL 对 SQL 语句进行脱敏处理
// 替换敏感信息（密码、手机号、身份证等）为 ***
func sanitizeSQL(sql string) string {
	// 脱敏密码字段
	sql = sanitizePassword(sql)

	// 脱敏手机号
	sql = sanitizePhone(sql)

	// 脱敏身份证号
	sql = sanitizeIDCard(sql)

	return sql
}

// sanitizePassword 脱敏密码字段
func sanitizePassword(sql string) string {
	// 匹配 password = 'xxx' 或 password='xxx'
	re := regexp.MustCompile(`(?i)(password\s*=\s*['"])([^'"]+)(['"])`)
	return re.ReplaceAllString(sql, `$1***$3`)
}

// sanitizePhone 脱敏手机号
func sanitizePhone(sql string) string {
	// 匹配 11 位手机号，保留前3位和后4位
	re := regexp.MustCompile(`(\d{3})\d{4}(\d{4})`)
	return re.ReplaceAllString(sql, `$1****$2`)
}

// sanitizeIDCard 脱敏身份证号
func sanitizeIDCard(sql string) string {
	// 匹配 18 位身份证号，保留前6位和后4位
	re := regexp.MustCompile(`(\d{6})\d{8}(\d{4})`)
	return re.ReplaceAllString(sql, `$1********$2`)
}

