import { lazy, Suspense, useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { Toaster } from '@/components/ui/sonner'
import { maybeRefreshToken } from '@/lib/api'
import AppLayout from '@/components/layout/AppLayout'
import DashboardPage from '@/pages/Dashboard'
import InstancesPage from '@/pages/Instances'
import LoginPage from '@/pages/Login'

// 重页面按路由懒加载,配合构建分包消除单 chunk 超 500kB 警告
const InstanceConfigPage = lazy(() => import('@/pages/InstanceConfig'))
const InstanceStatsPage = lazy(() => import('@/pages/InstanceStats'))
const MonitoringPage = lazy(() => import('@/pages/Monitoring'))
const AuditLogsPage = lazy(() => import('@/pages/AuditLogs'))
const UsersPage = lazy(() => import('@/pages/Users'))
const AlertsPage = lazy(() => import('@/pages/Alerts'))
const OidcCallbackPage = lazy(() => import('@/pages/OidcCallback'))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

function RouteFallback() {
  return (
    <div className="flex min-h-svh items-center justify-center">
      <Loader2 className="size-6 animate-spin text-muted-foreground" />
    </div>
  )
}

export default function App() {
  // 会话滑动续期:挂载时及每 30 分钟检查一次,余量不足 12h 自动换新 token
  useEffect(() => {
    maybeRefreshToken()
    const t = setInterval(() => maybeRefreshToken(), 30 * 60 * 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Suspense fallback={<RouteFallback />}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/oidc-callback" element={<OidcCallbackPage />} />
            <Route path="/" element={<AppLayout />}>
              <Route index element={<DashboardPage />} />
              <Route path="instances" element={<InstancesPage />} />
              <Route path="instances/:id/config" element={<InstanceConfigPage />} />
              <Route path="instances/:id/stats" element={<InstanceStatsPage />} />
              <Route path="monitoring" element={<MonitoringPage />} />
              <Route path="audit-logs" element={<AuditLogsPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="alerts" element={<AlertsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </BrowserRouter>
      <Toaster richColors position="top-center" />
    </QueryClientProvider>
  )
}
