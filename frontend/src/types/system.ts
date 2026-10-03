// Health probe shape from docs/API.md section 2.

export type HealthStatus = 'ok' | 'degraded' | 'down' | 'disabled'

export interface HealthComponent {
  status: HealthStatus
  required: boolean
  latency_ms?: number
  error?: string
}

export interface HealthReport {
  status: Exclude<HealthStatus, 'disabled'>
  timestamp: string
  components: Record<string, HealthComponent>
}
