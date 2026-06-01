import { Card, CardContent, CardHeader, Box, Typography, Switch, Chip, IconButton, Tooltip, CircularProgress } from '@mui/material'
import PowerSettingsNew from '@mui/icons-material/PowerSettingsNew'
import RefreshIcon from '@mui/icons-material/Refresh'
import Language from '@mui/icons-material/Language'
import CloudQueue from '@mui/icons-material/CloudQueue'
import { api, StatusResponse } from '../api'
import { useI18n } from '../i18n/I18nContext'

interface Props {
  status: StatusResponse | null
  loading: boolean
  onRefresh: () => void
  onError: (msg: string) => void
}

export default function ProxyControl({ status, loading, onRefresh, onError }: Props) {
  const { t } = useI18n()
  const running = status?.proxy_running ?? false

  const toggle = async () => {
    try {
      if (running) {
        await api.stopProxy()
      } else {
        await api.startProxy()
      }
      setTimeout(onRefresh, 500)
    } catch (e) {
      onError(e instanceof Error ? e.message : t('dashboard.toggle_failed'))
    }
  }

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title={t('dashboard.proxy')}
        titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
        action={
          <Tooltip title={t('dashboard.refresh')}>
            <IconButton size="small" onClick={onRefresh}>
              <RefreshIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        }
      />
      <CardContent sx={{ pt: 0 }}>
        <Box display="flex" alignItems="center" justifyContent="space-between" mb={2}>
          <Box display="flex" alignItems="center" gap={1.5}>
            <PowerSettingsNew
              sx={{
                fontSize: 40,
                color: running ? 'success.main' : 'action.active',
                filter: running ? 'drop-shadow(0 0 8px rgba(46, 160, 67, 0.5))' : 'none',
                transition: 'all 0.3s',
              }}
            />
            <Box>
              <Typography variant="h6" fontWeight={700}>
                {running ? t('dashboard.proxy_running') : t('dashboard.proxy_stopped')}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {status?.listen_addr || '—'}
              </Typography>
            </Box>
          </Box>
          {loading ? (
            <CircularProgress size={28} />
          ) : (
            <Switch
              checked={running}
              onChange={toggle}
              size="medium"
              color="success"
            />
          )}
        </Box>

        <Box display="flex" gap={1} flexWrap="wrap">
          <Chip
            icon={<Language sx={{ fontSize: 16 }} />}
            label={status?.proxy_mode?.toUpperCase() || 'MITM'}
            size="small"
            variant="outlined"
            color="primary"
          />
          <Chip
            icon={<CloudQueue sx={{ fontSize: 16 }} />}
            label={`SOCKS5 ${status?.socks5_enabled ? t('dashboard.socks5_on') : t('dashboard.socks5_off')}`}
            size="small"
            variant="outlined"
            color={status?.socks5_enabled ? 'secondary' : 'default'}
          />
        </Box>
      </CardContent>
    </Card>
  )
}
