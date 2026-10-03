// Types mirror docs/API.md. Field names match the Go `json` tags exactly.

/** Body of every non-2xx JSON response. */
export interface ApiErrorBody {
  error: {
    code: string
    message: string
    trace_id?: string
    details?: unknown
  }
}

export type Role = 'admin' | 'user'

export interface AuthUser {
  id: string
  role: Role
  tenant_id: number
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

export interface MeResponse {
  user: AuthUser
}

export type HealthStatus = 'ok' | 'degraded' | 'down' | 'disabled'

export interface HealthComponent {
  status: HealthStatus
  required: boolean
  latency_ms?: number
  error?: string
}

export interface HealthResponse {
  status: Exclude<HealthStatus, 'disabled'>
  timestamp: string
  components: Record<string, HealthComponent>
}
