import type { ConfigOp } from '@/types'

// describeOp 生成操作的人读摘要(暂存清单展示用)。
// 与后端 configOp.summary() 语义对齐;正式 note 仍以后端生成为准。
export function describeOp(op: ConfigOp): string {
  const port = op.port ?? '*'
  switch (op.kind) {
    case 'create_backend':
      return `创建 backend ${op.name}`
    case 'delete_backend':
      return `删除 backend ${op.name}`
    case 'create_server':
      return `在 ${op.backend} 添加服务器 ${op.name}(${op.address}:${port})`
    case 'update_server':
      return `修改 ${op.backend}/${op.name}(${op.address}:${port},检查=${op.check ?? 'disabled'})`
    case 'delete_server':
      return `删除服务器 ${op.backend}/${op.name}`
    case 'create_frontend':
      return `创建 frontend ${op.name}(${op.mode || 'http'} 模式,默认后端=${op.defaultBackend || '—'})`
    case 'update_frontend':
      return `frontend ${op.frontend} 默认后端改为 ${op.defaultBackend || '—'}`
    case 'delete_frontend':
      return `删除 frontend ${op.name}`
    case 'create_bind':
      return `frontend ${op.frontend} 添加监听 ${op.address}:${port}`
    case 'delete_bind':
      return `frontend ${op.frontend} 删除监听 ${op.name}`
    case 'create_acl': {
      const parent = op.parentType === 'backends' ? op.backend : op.frontend
      return `${parent ?? op.parentType} 添加 ACL ${op.aclName}(${op.criterion} ${op.value})`
    }
    case 'delete_acl':
      return `${op.parentType} 删除 ACL 组 ${op.aclName}`
    default:
      return op.kind
  }
}
