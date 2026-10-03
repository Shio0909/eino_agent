import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiGet } from '@/lib/api'
import { useAuth } from '@/store/auth'
import type { MeResponse } from '@/types/api'

/**
 * Lets signed-in users through and sends everyone else to /login.
 *
 * With auth disabled on the server, /auth/me answers without a token, so the
 * visitor is let in as the anonymous admin and never sees the login page. With
 * a stored token it also refreshes the user, which logs out an expired token.
 */
export function RequireAuth() {
  const token = useAuth((s) => s.token)
  const user = useAuth((s) => s.user)
  const setUser = useAuth((s) => s.setUser)
  const location = useLocation()
  const [checking, setChecking] = useState(!user)

  useEffect(() => {
    let cancelled = false
    apiGet<MeResponse>('/auth/me')
      .then((r) => !cancelled && setUser(r.user))
      .catch(() => {
        // 401 already cleared the session; other failures keep the stored user.
      })
      .finally(() => !cancelled && setChecking(false))
    return () => {
      cancelled = true
    }
  }, [token, setUser])

  if (checking) return <PageSpinner />
  if (!user) return <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
  return <Outlet />
}
