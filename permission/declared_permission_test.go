package permission

import "testing"

func TestNormalizeDeclaredPermissions(t *testing.T) {
	input := []DeclaredPermission{
		{PermissionCode: " role:read ", PermissionName: "查看角色", PermissionType: "read", ResourceCode: "role", GroupCode: "system"},
		{PermissionCode: "", PermissionName: "无效", PermissionType: "READ"},
		{PermissionCode: "role:read", PermissionName: "查看角色-v2", PermissionType: "READ", ResourceCode: "role", GroupCode: "SYSTEM"},
		{PermissionCode: "role:write", PermissionName: "管理角色", PermissionType: "write", ResourceCode: "role"},
	}

	result := NormalizeDeclaredPermissions(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}

	if result[0].PermissionCode != "role:read" {
		t.Fatalf("unexpected first permission code: %q", result[0].PermissionCode)
	}
	if result[0].PermissionName != "查看角色-v2" {
		t.Fatalf("expected latest duplicate overwrite, got %q", result[0].PermissionName)
	}
	if result[0].PermissionType != "READ" {
		t.Fatalf("expected uppercase permission type, got %q", result[0].PermissionType)
	}

	if result[1].GroupCode != "SYSTEM" {
		t.Fatalf("expected default group code SYSTEM, got %q", result[1].GroupCode)
	}
}
