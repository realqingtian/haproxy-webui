import { Badge } from '@/components/ui/badge'

// 按 HAProxy stats 的 status 字段前缀着色(UP/DOWN/MAINT/DRAIN 等)
export function StatusBadge({ status }: { status: string }) {
  if (status.startsWith('UP')) return <Badge className="bg-green-600">{status}</Badge>
  if (status.startsWith('DOWN')) return <Badge variant="destructive">{status}</Badge>
  if (status.includes('MAINT')) return <Badge variant="secondary">{status}</Badge>
  if (status.includes('DRAIN')) return <Badge className="bg-yellow-600">{status}</Badge>
  if (!status) return <Badge variant="outline">—</Badge>
  return <Badge variant="outline">{status}</Badge>
}
