import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import AppLayout from '@/components/layout/AppLayout'
import AuditLogsPage from '@/pages/AuditLogs'
import DashboardPage from '@/pages/Dashboard'
import InstanceConfigPage from '@/pages/InstanceConfig'
import InstanceStatsPage from '@/pages/InstanceStats'
import InstancesPage from '@/pages/Instances'
import LoginPage from '@/pages/Login'
import OidcCallbackPage from '@/pages/OidcCallback'
import UsersPage from '@/pages/Users'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/oidc-callback" element={<OidcCallbackPage />} />
          <Route path="/" element={<AppLayout />}>
            <Route index element={<DashboardPage />} />
            <Route path="instances" element={<InstancesPage />} />
            <Route path="instances/:id/config" element={<InstanceConfigPage />} />
            <Route path="instances/:id/stats" element={<InstanceStatsPage />} />
            <Route path="audit-logs" element={<AuditLogsPage />} />
            <Route path="users" element={<UsersPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
      <Toaster richColors position="top-center" />
    </QueryClientProvider>
  )
}
