export function fmtNum(v: unknown): string {
  const n = typeof v === 'number' ? v : parseFloat(String(v ?? ''))
  return Number.isFinite(n) ? n.toLocaleString('en-US') : '—'
}

export function fmtBytes(v: unknown): string {
  const n = typeof v === 'number' ? v : parseFloat(String(v ?? ''))
  if (!Number.isFinite(n)) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let val = Math.abs(n)
  let u = 0
  while (val >= 1024 && u < units.length - 1) {
    val /= 1024
    u++
  }
  return `${val >= 100 || u === 0 ? Math.round(val) : val.toFixed(1)} ${units[u]}`
}
