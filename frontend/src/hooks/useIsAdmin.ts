import { useAuth } from '@/store/auth'

/** True for the global admin role; used to hide admin-only controls. */
export function useIsAdmin(): boolean {
  return useAuth((s) => s.user?.role === 'admin')
}
