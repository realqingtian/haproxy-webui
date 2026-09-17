import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { api, ApiError, cacheCurrentUser, setToken } from '@/lib/api'
import type { LoginResponse } from '@/types'

export default function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [oidcEnabled, setOidcEnabled] = useState(false)

  // OIDC 是否已配置(未配置则不渲染 SSO 入口)
  useEffect(() => {
    api<{ enabled: boolean }>('/api/auth/oidc/status', {}, { authRedirect: false })
      .then((r) => setOidcEnabled(r.enabled))
      .catch(() => setOidcEnabled(false))
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      const resp = await api<LoginResponse>(
        '/api/auth/login',
        {
          method: 'POST',
          body: JSON.stringify({ username, password }),
        },
        { authRedirect: false },
      )
      setToken(resp.token)
      cacheCurrentUser(resp.user)
      toast.success(`欢迎回来,${resp.user.username}`)
      navigate('/', { replace: true })
    } catch (err) {
      let message = err instanceof ApiError ? err.message : '网络错误,请稍后重试'
      if (message === 'invalid credentials') {
        message = '用户名或密码错误'
      }
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-muted/40 px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl font-bold">HAProxy WebUI</CardTitle>
          <CardDescription>登录以管理你的 HAProxy 集群</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="username">用户名</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                autoFocus
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
              />
            </div>
          </CardContent>
          <CardFooter className="mt-6 flex-col gap-3">
            <Button type="submit" className="w-full" disabled={loading}>
              {loading && <Loader2 className="mr-2 size-4 animate-spin" />}
              登录
            </Button>
            {oidcEnabled && (
              <>
                <div className="flex w-full items-center gap-3">
                  <Separator className="flex-1" />
                  <span className="text-xs text-muted-foreground">或</span>
                  <Separator className="flex-1" />
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="w-full"
                  onClick={() => {
                    window.location.href = '/api/auth/oidc/start'
                  }}
                >
                  使用 SSO 登录
                </Button>
              </>
            )}
          </CardFooter>
        </form>
      </Card>
    </div>
  )
}
