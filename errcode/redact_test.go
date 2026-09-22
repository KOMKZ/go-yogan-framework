package errcode

import (
	"strings"
	"testing"
)

// 治理 ticket 000128 §1.2：Redact 是 cause/root/chain 写日志前的唯一通道，
// 覆盖 DSN、Bearer token、敏感键值、sk- 密钥、邮箱、手机号。
func TestRedactURLCredentials(t *testing.T) {
	cases := []string{
		"connect failed dsn=postgres://user:pass@host:5432/db",
		"redis dial tcp://admin:s3cret@127.0.0.1:6379/0",
		"request http://key:secret@api.example.com/v1",
	}
	for _, in := range cases {
		out := Redact(in)
		if strings.Contains(out, "pass@") || strings.Contains(out, "s3cret@") || strings.Contains(out, "secret@") {
			t.Fatalf("Redact(%q) = %q, password survived", in, out)
		}
		if !strings.Contains(out, ":***@") {
			t.Fatalf("Redact(%q) = %q, want masked credential", in, out)
		}
	}
}

// 000128 review 整改 A：密码包含 @ 时不得残留任何片段（旧正则在第一个 @ 截断，
// `postgres://admin:p@ssw0rd@db` 会漏掉 `ssw0rd@db`）。
func TestRedactURLCredentialsPasswordWithAtAndEncoding(t *testing.T) {
	cases := []struct {
		in       string
		leakFrag []string // 任何片段出现在结果里都算泄漏
	}{
		{"connect dsn=postgres://admin:p@ssw0rd@db:5432/core failed", []string{"ssw0rd", "p@ss"}},
		{"mysql://root:pa:ss@word@10.0.0.1:3306/app", []string{"pa:ss", "@word@"}},
		{"pg://u:p%40ss%3Aw0rd@host/db", []string{"%40ss", "w0rd"}},
		{"http://key:deadbeef@inner@api.example.com/v1", []string{"deadbeef"}},
	}
	for _, tc := range cases {
		out := Redact(tc.in)
		for _, frag := range tc.leakFrag {
			if strings.Contains(out, frag) {
				t.Fatalf("Redact(%q) = %q, fragment %q survived", tc.in, out, frag)
			}
		}
		if !strings.Contains(out, ":***@") {
			t.Fatalf("Redact(%q) = %q, want masked credential", tc.in, out)
		}
	}
	// 无密码形态（scheme://user@host，userinfo 无冒号）不应被掩码。
	// 注：git@github.com 这类会被邮箱规则掩码本地部分——那是邮箱 PII 规则的预期行为。
	if out := Redact("redis://default@cache:6379/0 connection refused"); !strings.Contains(out, "redis://default@cache") {
		t.Fatalf("password-less userinfo must stay intact, got %q", out)
	}
}

func TestRedactBearerToken(t *testing.T) {
	in := "Unauthorized: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.tail"
	out := Redact(in)
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Fatalf("Bearer token survived: %q", out)
	}
	if !strings.Contains(out, "Bearer ***") {
		t.Fatalf("Bearer marker lost: %q", out)
	}
}

func TestRedactSensitiveKeyValues(t *testing.T) {
	cases := map[string]string{
		"token=abc123&user=1":                  "token=abc123",
		"access_token: zzz.yyy.xxx; expires=1": "zzz.yyy.xxx",
		"password = mypass123":                 "mypass123",
		"api_key=AKIA1234567890":               "AKIA1234567890",
		"signature=deadbeef01":                 "deadbeef01",
		`{"api_key": "sk-verysecret1"}`:        "sk-verysecret1",
	}
	for in, leaked := range cases {
		out := Redact(in)
		if strings.Contains(out, leaked) {
			t.Fatalf("Redact(%q) = %q, secret %q survived", in, out, leaked)
		}
	}
}

func TestRedactSignedMediaURLQuery(t *testing.T) {
	in := "stat https://cdn.example.test/video.mp4?auth_key=1700000000-deadbeef&OSSAccessKeyId=test-access-id&Signature=test-signature: no such file"
	out := Redact(in)
	for _, leaked := range []string{"1700000000-deadbeef", "test-access-id", "test-signature"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("Redact signed URL = %q, secret %q survived", out, leaked)
		}
	}
	for _, masked := range []string{"auth_key=***", "OSSAccessKeyId=***", "Signature=***"} {
		if !strings.Contains(out, masked) {
			t.Fatalf("Redact signed URL = %q, want %q", out, masked)
		}
	}
}

func TestRedactSecretKey(t *testing.T) {
	in := `auth failed with key sk-9f8e7d6c5b4a3210fedcba`
	out := Redact(in)
	if strings.Contains(out, "sk-9f8e7d6c5b4a3210fedcba") {
		t.Fatalf("sk- key survived: %q", out)
	}
	if !strings.Contains(out, "sk-***") {
		t.Fatalf("sk- mask missing: %q", out)
	}
}

func TestRedactEmailKeepsDomain(t *testing.T) {
	out := Redact("mail to zhang.san@example.com failed")
	if strings.Contains(out, "zhang.san") {
		t.Fatalf("email local part survived: %q", out)
	}
	if !strings.Contains(out, "***@example.com") {
		t.Fatalf("email domain should stay: %q", out)
	}
}

func TestRedactMobile(t *testing.T) {
	out := Redact("sms to 13812345678 rejected")
	if strings.Contains(out, "13812345678") {
		t.Fatalf("mobile survived: %q", out)
	}
	if !strings.Contains(out, "138****5678") {
		t.Fatalf("mobile mask wrong: %q", out)
	}
	// 非 11 位独立数字不得误伤：订单号等长数字串。
	keep := Redact("order 213812345678 total 42")
	if !strings.Contains(keep, "213812345678") {
		t.Fatalf("long digit run wrongly masked: %q", keep)
	}
}

func TestRedactSafeTextUnchanged(t *testing.T) {
	in := "model qwen-image-2.0 returns invalid_request, verify_code=F008"
	if out := Redact(in); out != in {
		t.Fatalf("plain technical text should be unchanged, got %q", out)
	}
	if out := Redact(""); out != "" {
		t.Fatalf("empty text should stay empty, got %q", out)
	}
}
