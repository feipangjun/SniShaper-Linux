import { useState, useEffect } from 'react'
import {
  Card, CardContent, CardHeader, Box, Typography, TextField, Switch, Button,
  Select, MenuItem, FormControl, InputLabel, Alert, Snackbar, Table, TableBody, TableCell,
  TableHead, TableRow, Chip,
} from '@mui/material'
import Save from '@mui/icons-material/Save'
import CloudDownload from '@mui/icons-material/CloudDownload'
import HealthAndSafety from '@mui/icons-material/HealthAndSafety'
import FileDownload from '@mui/icons-material/FileDownload'
import FileUpload from '@mui/icons-material/FileUpload'
import Refresh from '@mui/icons-material/Refresh'
import { api, ConfigResponse, CFIPStats } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function SettingsPanel() {
  const { t, lang, setLang } = useI18n()
  const [config, setConfig] = useState<ConfigResponse | null>(null)
  const [saved, setSaved] = useState(false)
  const [cfStats, setCfStats] = useState<CFIPStats[]>([])

  useEffect(() => {
    api.getConfig().then(setConfig).catch(() => {})
    api.getCFStats().then(setCfStats).catch(() => {})
  }, [])

  const saveConfig = async () => {
    if (!config) return
    await api.updateConfig(config)
    setSaved(true)
  }

  if (!config) return <Typography>{t('common.loading')}</Typography>

  return (
    <>
      <Snackbar open={saved} autoHideDuration={3000} onClose={() => setSaved(false)}>
        <Alert severity="success">{t('settings.saved')}</Alert>
      </Snackbar>

      <Box display="flex" flexDirection="column" gap={2}>
        <Card>
          <CardHeader title={t('settings.title')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" flexDirection="column" gap={2} maxWidth={500}>
              <TextField label={t('settings.http_port')} value={config.listen_port || '8080'}
                onChange={e => setConfig({ ...config, listen_port: e.target.value })} size="small" />
              <Box display="flex" alignItems="center" gap={2}>
                <Switch checked={config.socks5_enabled}
                  onChange={e => setConfig({ ...config, socks5_enabled: e.target.checked })} />
                <Typography variant="body2">{t('settings.socks5_enable')}</Typography>
              </Box>
              {config.socks5_enabled && (
                <TextField label={t('settings.socks5_port')} value={config.socks5_port || '8081'}
                  onChange={e => setConfig({ ...config, socks5_port: e.target.value })} size="small" />
              )}
              <FormControl size="small">
                <InputLabel>{t('settings.proxy_mode')}</InputLabel>
                <Select value={config.proxy_mode || 'mitm'} label={t('settings.proxy_mode')}
                  onChange={e => setConfig({ ...config, proxy_mode: e.target.value })}>
                  <MenuItem value="mitm">{t('settings.mode_mitm')}</MenuItem>
                  <MenuItem value="server">{t('settings.mode_server')}</MenuItem>
                  <MenuItem value="tls-rf">{t('settings.mode_tlsrf')}</MenuItem>
                  <MenuItem value="quic">{t('settings.mode_quic')}</MenuItem>
                  <MenuItem value="transparent">{t('settings.mode_transparent')}</MenuItem>
                </Select>
              </FormControl>
              <Button variant="contained" startIcon={<Save />} onClick={saveConfig} sx={{ alignSelf: 'flex-start' }}>
                {t('settings.save')}
              </Button>
            </Box>
          </CardContent>
        </Card>

        <Card>
          <CardHeader title="TUN" titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" flexDirection="column" gap={2} maxWidth={500}>
              <TextField label="MTU" value={config.tun?.mtu ?? 1500}
                onChange={e => setConfig({ ...config, tun: { ...config.tun, mtu: Number(e.target.value) } })}
                size="small" type="number" />
              <Box display="flex" alignItems="center" gap={2}>
                <Switch checked={config.tun?.dns_hijack ?? false}
                  onChange={e => setConfig({ ...config, tun: { ...config.tun, dns_hijack: e.target.checked } })} />
                <Typography variant="body2">DNS Hijack</Typography>
              </Box>
              <Box display="flex" alignItems="center" gap={2}>
                <Switch checked={config.tun?.auto_route ?? false}
                  onChange={e => setConfig({ ...config, tun: { ...config.tun, auto_route: e.target.checked } })} />
                <Typography variant="body2">Auto Route</Typography>
              </Box>
              <Box display="flex" alignItems="center" gap={2}>
                <Switch checked={config.tun?.strict_route ?? false}
                  onChange={e => setConfig({ ...config, tun: { ...config.tun, strict_route: e.target.checked } })} />
                <Typography variant="body2">Strict Route</Typography>
              </Box>
            </Box>
          </CardContent>
        </Card>

        <Card>
          <CardHeader title={t('settings.language')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" gap={2} alignItems="center">
              <Button variant={lang === 'zh' ? 'contained' : 'outlined'}
                onClick={() => { setLang('zh'); api.updateConfig({ language: 'zh' }) }}>
                {t('settings.language_zh')}
              </Button>
              <Button variant={lang === 'en' ? 'contained' : 'outlined'}
                onClick={() => { setLang('en'); api.updateConfig({ language: 'en' }) }}>
                {t('settings.language_en')}
              </Button>
              <Button variant={lang === 'ru' ? 'contained' : 'outlined'}
                onClick={() => { setLang('ru'); api.updateConfig({ language: 'ru' }) }}>
                Русский
              </Button>
            </Box>
          </CardContent>
        </Card>

        <Card>
          <CardHeader title={t('settings.server_node')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" flexDirection="column" gap={2} maxWidth={500}>
              <TextField label={t('settings.server_host')} value={config.server_host || ''} size="small"
                onChange={e => setConfig({ ...config, server_host: e.target.value })} />
              <TextField label={t('settings.server_auth')} value={config.server_auth || ''} size="small" type="password"
                onChange={e => setConfig({ ...config, server_auth: e.target.value })} />
              <Button variant="outlined" startIcon={<Save />} onClick={() => {
                api.updateServerConfig(config.server_host || '', config.server_auth || '')
                setSaved(true)
              }} sx={{ alignSelf: 'flex-start' }}>
                {t('settings.server_save')}
              </Button>
            </Box>
          </CardContent>
        </Card>

        <Card>
          <CardHeader title={t('settings.cf_pool')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" gap={1} mb={2}>
              <Button variant="outlined" startIcon={<CloudDownload />} onClick={async () => {
                await api.fetchCFIPs()
                api.getCFStats().then(setCfStats)
              }}>
                {t('settings.cf_fetch')}
              </Button>
              <Button variant="outlined" startIcon={<HealthAndSafety />} onClick={async () => {
                await api.triggerCFHealthCheck()
                setTimeout(() => api.getCFStats().then(setCfStats), 2000)
              }}>
                {t('settings.cf_health')}
              </Button>
            </Box>
            {cfStats.length > 0 && (
              <Table size="small" sx={{ maxHeight: 300 }}>
                <TableHead>
                  <TableRow>
                    <TableCell>IP</TableCell>
                    <TableCell>Latency</TableCell>
                    <TableCell>Failures</TableCell>
                    <TableCell>Last Check</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {cfStats.map(s => (
                    <TableRow key={s.ip}>
                      <TableCell><Typography variant="caption" fontFamily="monospace">{s.ip}</Typography></TableCell>
                      <TableCell>
                        <Chip size="small" label={s.latency || '-'}
                          color={s.latency !== '' && s.latency !== '0s' ? 'success' : 'default'} variant="outlined" />
                      </TableCell>
                      <TableCell>{s.failures}</TableCell>
                      <TableCell><Typography variant="caption" color="text.secondary">{s.last_check || '-'}</Typography></TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader title="CA" titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" gap={1}>
              <Button variant="outlined" startIcon={<FileDownload />} onClick={() => window.open('/api/cert/export', '_blank')}>
                {t('settings.ca_export')}
              </Button>
              <Button variant="outlined" color="warning" startIcon={<Refresh />} onClick={async () => {
                await api.regenerateCert()
                setSaved(true)
              }}>
                {t('settings.ca_reset')}
              </Button>
            </Box>
          </CardContent>
        </Card>

        <Card>
          <CardHeader title="Config" titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" gap={1}>
              <Button variant="outlined" startIcon={<FileUpload />} onClick={async () => {
                const data = await api.exportConfig()
                const blob = new Blob([data.config], { type: 'application/json' })
                const url = URL.createObjectURL(blob)
                const a = document.createElement('a')
                a.href = url; a.download = 'snishaper-config.json'; a.click()
                URL.revokeObjectURL(url)
              }}>
                {t('settings.config_export')}
              </Button>
              <Button variant="outlined" startIcon={<FileDownload />} component="label">
                {t('settings.config_import')}
                <input type="file" hidden accept=".json" onChange={async e => {
                  const file = e.target.files?.[0]
                  if (!file) return
                  const text = await file.text()
                  await api.importConfig(text)
                  setSaved(true)
                }} />
              </Button>
            </Box>
          </CardContent>
        </Card>
      </Box>
    </>
  )
}
