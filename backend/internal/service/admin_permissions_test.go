//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAdminPermissions(t *testing.T) {
	got, err := NormalizeAdminPermissions([]string{
		" admin.users.write ",
		AdminPermissionUsersRead,
		AdminPermissionUsersRead,
		"",
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		AdminPermissionUsersRead,
		AdminPermissionUsersWrite,
	}, got)
}
func TestNormalizeAdminPermissionsRejectsUnknownPermission(t *testing.T) {
	got, err := NormalizeAdminPermissions([]string{"admin.users.read", "admin.unknown.read"})
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "unknown admin permission")
}

func TestAdminPermissionDefinitionsCoverKnownPermissions(t *testing.T) {
	defs := AdminPermissionDefinitions()
	require.NotEmpty(t, defs)

	keys := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		require.NotEmpty(t, def.Key)
		require.NotEmpty(t, def.Module)
		require.NotEmpty(t, def.Action)
		require.NotEmpty(t, def.Label)
		require.True(t, IsKnownAdminPermission(def.Key), def.Key)
		keys[def.Key] = struct{}{}
	}

	require.Len(t, keys, len(defs), "permission keys must be unique")
	require.Contains(t, keys, AdminPermissionUsersRead)
	require.Contains(t, keys, AdminPermissionPaymentWrite)
}
