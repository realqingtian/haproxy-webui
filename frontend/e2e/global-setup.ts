import { execSync } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

// globalSetup:确保 local-e2e 容器(真实 HAProxy + dataplaneapi v3)就绪。
// Docker 不可用时给出明确报错而不是晦涩的连接失败。
export default function globalSetup() {
  const here = dirname(fileURLToPath(import.meta.url))
  const composeFile = resolve(here, '../../deploy/dataplaneapi/local-e2e/docker-compose.yml')
  try {
    execSync(`docker compose -f "${composeFile}" up -d --wait`, {
      stdio: 'inherit',
      timeout: 180_000,
    })
  } catch (err) {
    throw new Error(
      `E2E 依赖 local-e2e 容器,启动失败(${composeFile})。请确认 Docker 已运行后重试。原因: ${err}`,
    )
  }
}
