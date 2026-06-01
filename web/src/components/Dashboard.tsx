import { useState, useEffect, useCallback } from 'react'
import { Grid2 as Grid, Alert, Snackbar } from '@mui/material'
import { api, StatusResponse, StatsResponse } from '../api'
import { useI18n } from '../i18n/I18nContext'
import ProxyControl from './ProxyControl'
import TrafficStats from './TrafficStats'
import CertPanel from './CertPanel'
import TUNPanel from './TUNPanel'
import LogViewer from './LogViewer'
import RulesPanel from './RulesPanel'

export default function Dashboard() {
  const { t } = useI18n()
  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [stats, setStats] = useState<StatsResponse>({ bytes_down: 0, bytes_up: 0 })
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const refreshStatus = useCallback(async () => {
    try {
      const [s, st] = await Promise.all([api.getStatus(), api.getStats()])
      setStatus(s)
      setStats(st)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : t('dashboard.refresh_failed'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    refreshStatus()
    const timer = setInterval(refreshStatus, 3000)
    return () => clearInterval(timer)
  }, [refreshStatus])

  return (
    <>
      <Snackbar open={!!error} autoHideDuration={6000} onClose={() => setError(null)}>
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      </Snackbar>

      <Grid container spacing={2.5}>
        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <ProxyControl
            status={status}
            loading={loading}
            onRefresh={refreshStatus}
            onError={setError}
          />
        </Grid>

        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <TrafficStats stats={stats} />
        </Grid>

        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <CertPanel onError={setError} />
        </Grid>

        <Grid size={{ xs: 12, md: 6, lg: 4 }}>
          <TUNPanel
            tunStatus={status?.tun}
            onError={setError}
            onRefresh={refreshStatus}
          />
        </Grid>

        <Grid size={{ xs: 12, lg: 8 }}>
          <LogViewer />
        </Grid>

        <Grid size={{ xs: 12 }}>
          <RulesPanel />
        </Grid>
      </Grid>
    </>
  )
}
