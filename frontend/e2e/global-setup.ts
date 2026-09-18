import { execSync } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

// globalSetup:确保 local-e2e 容器(真实 HAProxy + dataplaneapi v3)就绪。
// Docker 不可用时给出明确报错而不是晦涩的连接失败。
export default function globalSetup() {
  const here = dirname(fileURLToPath(import.meta.url))
  const composeFile = resolve(here, '../../deploy/dataplaneapi/local-e2e/docker-compose.yml')
  try {
    // --build:镜像里 COPY 了 start.sh 等源文件,源码改动后必须重建(层缓存,秒级)
    execSync(`docker compose -f "${composeFile}" up -d --build --wait`, {
      stdio: 'inherit',
      timeout: 300_000,
    })
  } catch (err) {
    throw new Error(
      `E2E 依赖 local-e2e 容器,启动失败(${composeFile})。请确认 Docker 已运行后重试。原因: ${err}`,
    )
  }
}
