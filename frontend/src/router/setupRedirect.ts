export function resolveCompletedSetupRedirectPath(isAuthenticated: boolean, canAccessAdmin: boolean): string {
  if (!isAuthenticated) {
    return '/login'
  }

  return canAccessAdmin ? '/admin/dashboard' : '/dashboard'
}
