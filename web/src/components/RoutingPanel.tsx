import { useState, useEffect } from 'react'
import {
  Card, CardContent, CardHeader, Box, Typography, Button, FormControl, InputLabel, Select, MenuItem, Chip,
} from '@mui/material'
import Save from '@mui/icons-material/Save'
import Refresh from '@mui/icons-material/Refresh'
import AutoFixHigh from '@mui/icons-material/AutoFixHigh'
import { api } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function RoutingPanel() {
  const { t } = useI18n()
  const [mode, setMode] = useState('')
  const [gfwStatus, setGfwStatus] = useState<{ enabled: boolean; domain_count: number }>({ enabled: false, domain_count: 0 })
  const [refreshing, setRefreshing] = useState(false)

  useEffect(() => {
    api.getAutoRouteConfig().then(cfg => {
      if (cfg.mode) setMode(cfg.mode)
    }).catch(() => {})
    api.getAutoRouteStatus().then(s => setGfwStatus(s)).catch(() => {})
  }, [])

  const saveRouting = async () => {
    await api.updateAutoRouteConfig(mode)
  }

  const refreshGfw = async () => {
    setRefreshing(true)
    try {
      await api.refreshGFWList()
      const s = await api.getAutoRouteStatus()
      setGfwStatus(s)
    } finally { setRefreshing(false) }
  }

  return (
    <Card>
      <CardHeader avatar={<AutoFixHigh />} title={t('routing.title')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
      <CardContent sx={{ pt: 0 }}>
        <Box display="flex" flexDirection="column" gap={3} maxWidth={500}>
          <FormControl size="small" fullWidth>
            <InputLabel>{t('routing.mode')}</InputLabel>
            <Select value={mode} label={t('routing.mode')} onChange={e => setMode(e.target.value)}>
              <MenuItem value="">{t('routing.mode_off')}</MenuItem>
              <MenuItem value="default">{t('routing.mode_smart')}</MenuItem>
              <MenuItem value="server">{t('routing.mode_server')}</MenuItem>
            </Select>
          </FormControl>

          <Box display="flex" alignItems="center" gap={2}>
            <Typography variant="body2">{t('routing.gfwlist')}: </Typography>
            <Chip
              size="small"
              label={gfwStatus.enabled ? `${gfwStatus.domain_count} 条规则` : t('routing.gfwlist_inactive')}
              color={gfwStatus.enabled ? 'success' : 'default'}
            />
            <Button size="small" variant="outlined" startIcon={<Refresh />} onClick={refreshGfw} disabled={refreshing}>
              {t('routing.refresh_gfwlist')}
            </Button>
          </Box>

          <Button variant="contained" startIcon={<Save />} onClick={saveRouting} sx={{ alignSelf: 'flex-start' }}>
            {t('routing.save')}
          </Button>
        </Box>
      </CardContent>
    </Card>
  )
}
