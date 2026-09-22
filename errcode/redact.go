// redact.go 提供错误文本统一脱敏器（治理 ticket 000128 §1.2）。
//
// cause.Error()、root.Error()、error_chain 等技术详情只允许经过 Redact 后写入
// 日志或诊断字段：客户端安全不等于日志安全，DSN、token、签名、手机号、邮箱等
// 泄漏进日志同样构成安全事故。脱敏测试是框架回归测试的一部分（redact_test.go）。
package errcode

import (
	"regexp"
	"strings"
)

var (
	// URL 携带基本凭据：postgres://user:password@host、http://key:secret@host 等。
	// 密码允许包含 @ 与 URL 编码（RFC 3986：最后一个 @ 分隔 userinfo 与 host），
	// 因此这里只定位 "scheme://...@" 前缀，掩码在回调函数内完成。
	redactURLCredentials = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*)://[^\s/]*@`)
	// Bearer / Authorization 头。
	redactBearer = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]+`)
	// query/form 风格 key=value（token=xxx、api_key: xxx、auth_key=xxx、password = xxx）。
	// key 按 longest-first 排列，避免 access_token 被 token 先截断。
	redactKeyValue = regexp.MustCompile(`(?i)\b(ossaccesskeyid|access_key_id|accesskeyid|access_token|refresh_token|session_key|auth_key|api_key|apikey|secret|signature|password|passwd|pwd|credential|authorization|token)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s&,;}\]]+)`)
	// JSON 风格 "api_key": "xxx"。
	redactJSONValue = regexp.MustCompile(`(?i)("(?:ossaccesskeyid|access_key_id|accesskeyid|access_token|refresh_token|session_key|auth_key|api_key|apikey|secret|signature|password|passwd|pwd|credential|authorization|token)"\s*:\s*)"[^"]*"`)
	// DashScope / OpenAI 风格 sk- 密钥。
	redactSecretKey = regexp.MustCompile(`\bsk-[A-Za-z0-9]{8,}\b`)
	// 邮箱：保留域名，掩码本地部分。
	redactEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// 中国大陆手机号：保留前 3 后 4。
	redactMobile = regexp.MustCompile(`\b1[3-9][0-9]{9}\b`)
)

// Redact 对错误文本做统一脱敏，返回可安全写入日志/诊断的文本。
//
// 覆盖：URL 基本凭据（DSN，密码可含 @/冒号/URL 编码）、Bearer token、
// query/form/JSON 风格的 token/secret/api_key/auth_key/OSSAccessKeyId/signature/password 等敏感键值、
// sk- 风格密钥、邮箱、手机号。任何 cause/root/chain 文本写日志前必须经过本函数。
func Redact(text string) string {
	if text == "" {
		return ""
	}
	out := redactURLCredentials.ReplaceAllStringFunc(text, redactURLCredentialMatch)
	out = redactBearer.ReplaceAllString(out, "${1}***")
	out = redactJSONValue.ReplaceAllString(out, `${1}"***"`)
	out = redactKeyValue.ReplaceAllString(out, "${1}${2}***")
	out = redactSecretKey.ReplaceAllString(out, "sk-***")
	out = redactEmail.ReplaceAllStringFunc(out, func(match string) string {
		at := strings.LastIndex(match, "@")
		return "***" + match[at:]
	})
	out = redactMobile.ReplaceAllStringFunc(out, func(match string) string {
		return match[:3] + "****" + match[7:]
	})
	return out
}

// redactURLCredentialMatch 掩码单个 "scheme://userinfo@" 匹配：
// 按最后一个 @ 取 userinfo，再按第一个冒号拆 user:password——密码整段替换为 ***，
// 无论其中包含 @、冒号还是 URL 编码都不会残留片段。
func redactURLCredentialMatch(match string) string {
	schemeEnd := strings.Index(match, "://")
	at := strings.LastIndex(match, "@")
	if schemeEnd < 0 || at < schemeEnd {
		return match
	}
	userinfo := match[schemeEnd+3 : at]
	colon := strings.Index(userinfo, ":")
	if colon < 0 {
		// 无密码形态（scheme://user@host）：无可掩码凭据。
		return match
	}
	return match[:schemeEnd+3] + userinfo[:colon] + ":***" + match[at:]
}
