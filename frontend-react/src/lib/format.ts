export function formatDate(value?: string) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

export function compactNumber(value?: number) {
  if (value == null) return '0';
  return new Intl.NumberFormat('zh-CN', { notation: value > 9999 ? 'compact' : 'standard' }).format(value);
}

export function statusTone(status?: string) {
  const normalized = status?.toLowerCase() ?? '';
  if (['done', 'completed', 'success', 'ready', 'indexed'].includes(normalized)) return 'success';
  if (['failed', 'error'].includes(normalized)) return 'error';
  if (['running', 'processing', 'pending', 'queued', 'importing'].includes(normalized)) return 'warning';
  return 'muted';
}
