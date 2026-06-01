import { useState } from 'react'
import { Card, CardContent, CardHeader, Box, Typography, Chip, Button, CircularProgress } from '@mui/material'
import Router from '@mui/icons-material/Router'
import CheckCircle from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import Info from '@mui/icons-material/Info'
import PlayArrow from '@mui/icons-material/PlayArrow'
import Stop from '@mui/icons-material/Stop'
import { StatusResponse, api } from '../api'
import { useI18n } from '../i18n/I18nContext'

interface Props {
  tunStatus?: StatusResponse['tun']
  onError: (msg: string) => void
  onRefresh: () => void
}

export default function TUNPanel({ tunStatus, onError, onRefresh }: Props) {
  const { t } = useI18n()
  const [loading, setLoading] = useState(false)
  const running = tunStatus?.running ?? false
  const supported = tunStatus?.supported ?? false

  const handleStart = async () => {
    setLoading(true)
    try {
      await api.startTUN()
      setTimeout(onRefresh, 1000)
    } catch (e) {
      onError(e instanceof Error ? e.message : 'TUN start failed')
    } finally {
      setLoading(false)
    }
  }

  const handleStop = async () => {
    setLoading(true)
    try {
      await api.stopTUN()
      setTimeout(onRefresh, 1000)
    } catch (e) {
      onError(e instanceof Error ? e.message : 'TUN stop failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title={t('dashboard.tun_mode')}
        titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
      />
      <CardContent sx={{ pt: 0 }}>
        <Box display="flex" alignItems="center" gap={1.5} mb={2}>
          <Router
            sx={{
              fontSize: 32,
              color: running ? 'success.main' : 'action.active',
              filter: running ? 'drop-shadow(0 0 8px rgba(46, 160, 67, 0.5))' : 'none',
            }}
          />
          <Box>
            <Typography variant="h6" fontWeight={700}>
              {running ? t('dashboard.tun_running') : t('dashboard.tun_stopped')}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {tunStatus?.driver || 'mihomo'}
            </Typography>
          </Box>
        </Box>

        <Box display="flex" gap={1} mb={1.5} flexWrap="wrap">
          {running ? (
            <Chip icon={<CheckCircle />} label={t('dashboard.active')} color="success" size="small" />
          ) : (
            <Chip icon={<ErrorIcon />} label={t('dashboard.inactive')} color="default" size="small" />
          )}
          {supported ? (
            <Chip icon={<Info />} label={t('dashboard.supported')} color="info" size="small" variant="outlined" />
          ) : (
            <Chip label={t('dashboard.not_supported')} color="warning" size="small" variant="outlined" />
          )}
        </Box>

        {supported && (
          <Box display="flex" gap={1} mb={1}>
            {running ? (
              <Button size="small" variant="outlined" color="error" startIcon={<Stop />} onClick={handleStop} disabled={loading}>
                {loading ? <CircularProgress size={14} /> : t('dashboard.stop')}
              </Button>
            ) : (
              <Button size="small" variant="contained" startIcon={<PlayArrow />} onClick={handleStart} disabled={loading}>
                {loading ? <CircularProgress size={14} /> : t('dashboard.start')}
              </Button>
            )}
          </Box>
        )}

        {tunStatus?.message && (
          <Typography variant="caption" color="text.secondary" display="block">
            {tunStatus.message}
          </Typography>
        )}

        {!supported && (
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
            {t('dashboard.tun_cli_hint')}
          </Typography>
        )}
      </CardContent>
    </Card>
  )
}
