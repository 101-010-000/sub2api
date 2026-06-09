package middleware

import "github.com/gin-gonic/gin"

// AuthSubject is the minimal authenticated identity stored in gin context.
// Decision: {UserID int64, Concurrency int}
type AuthSubject struct {
	UserID      int64
	Concurrency int
}

func GetAuthSubjectFromContext(c *gin.Context) (AuthSubject, bool) {
	value, exists := c.Get(string(ContextKeyUser))
	if !exists {
		return AuthSubject{}, false
	}
	subject, ok := value.(AuthSubject)
	return subject, ok
}

func GetUserRoleFromContext(c *gin.Context) (string, bool) {
	value, exists := c.Get(string(ContextKeyUserRole))
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}

func GetAdminPermissionsFromContext(c *gin.Context) []string {
	value, exists := c.Get(string(ContextKeyAdminPermissions))
	if !exists {
		return nil
	}
	permissions, _ := value.([]string)
	return permissions
}

func IsSuperAdminContext(c *gin.Context) bool {
	value, exists := c.Get(string(ContextKeyAdminSuper))
	if !exists {
		return false
	}
	ok, _ := value.(bool)
	return ok
}

func CanAccessAdminContext(c *gin.Context) bool {
	if IsSuperAdminContext(c) {
		return true
	}
	return len(GetAdminPermissionsFromContext(c)) > 0
}
