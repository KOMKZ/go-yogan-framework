package permission

import "strings"

// DeclaredPermission defines a domain-owned permission dictionary item.
type DeclaredPermission struct {
	PermissionCode string
	PermissionName string
	PermissionType string
	ResourceCode   string
	GroupCode      string
	Description    string
}

// NormalizeDeclaredPermissions trims fields and deduplicates by permission code.
func NormalizeDeclaredPermissions(items []DeclaredPermission) []DeclaredPermission {
	if len(items) == 0 {
		return nil
	}

	index := make(map[string]DeclaredPermission, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		normalized := DeclaredPermission{
			PermissionCode: strings.TrimSpace(item.PermissionCode),
			PermissionName: strings.TrimSpace(item.PermissionName),
			PermissionType: strings.ToUpper(strings.TrimSpace(item.PermissionType)),
			ResourceCode:   strings.TrimSpace(item.ResourceCode),
			GroupCode:      strings.TrimSpace(item.GroupCode),
			Description:    strings.TrimSpace(item.Description),
		}
		if normalized.PermissionCode == "" || normalized.PermissionName == "" || normalized.PermissionType == "" {
			continue
		}
		if normalized.ResourceCode == "" {
			normalized.ResourceCode = normalized.PermissionCode
		}
		if normalized.GroupCode == "" {
			normalized.GroupCode = "SYSTEM"
		}

		if _, exists := index[normalized.PermissionCode]; !exists {
			order = append(order, normalized.PermissionCode)
		}
		index[normalized.PermissionCode] = normalized
	}

	result := make([]DeclaredPermission, 0, len(order))
	for _, code := range order {
		result = append(result, index[code])
	}
	return result
}
