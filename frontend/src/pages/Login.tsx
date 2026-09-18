import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      toast.success(t('login.welcome', { name: resp.user.username }))
      navigate('/', { replace: true })
    } catch (err) {
      let message = err instanceof ApiError ? err.message : t('login.networkError')
      if (message === 'invalid credentials') {
        message = t('login.badCredentials')
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
          <CardDescription>{t('login.subtitle')}</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="username">{t('login.username')}</Label>
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
              <Label htmlFor="password">{t('login.password')}</Label>
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
              {t('login.submit')}
            </Button>
            {oidcEnabled && (
              <>
                <div className="flex w-full items-center gap-3">
                  <Separator className="flex-1" />
                  <span className="text-xs text-muted-foreground">{t('login.or')}</span>
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
                  {t('login.sso')}
                </Button>
              </>
            )}
          </CardFooter>
        </form>
      </Card>
    </div>
  )
}
