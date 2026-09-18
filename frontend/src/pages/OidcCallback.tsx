import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { api, ApiError, cacheCurrentUser, setToken } from '@/lib/api'
import type { User } from '@/types'

// OIDC 授权完成后的前端落点:从 URL 取本系统 JWT,换取用户信息后进入系统
export default function OidcCallbackPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [params] = useSearchParams()

  useEffect(() => {
    const token = params.get('token')
    const error = params.get('error')
    if (error) {
      toast.error(error)
      navigate('/login', { replace: true })
      return
    }
    if (!token) {
      navigate('/login', { replace: true })
      return
    }
    setToken(token)
    api<User>('/api/auth/me')
      .then((user) => {
        cacheCurrentUser(user)
        toast.success(t('login.welcome', { name: user.username }))
        navigate('/', { replace: true })
      })
      .catch((e) => {
        toast.error(e instanceof ApiError ? e.message : t('oidc.userFailed'))
        navigate('/login', { replace: true })
      })
  }, [params, navigate])

  return (
    <div className="flex min-h-svh items-center justify-center bg-muted/40">
      <Card className="w-72">
        <CardContent className="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground">
          <Loader2 className="size-5 animate-spin" />
          {t('oidc.completing')}
        </CardContent>
      </Card>
    </div>
  )
}
