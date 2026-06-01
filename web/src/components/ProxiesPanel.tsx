import { useState, useEffect } from 'react'
import {
  Card, CardContent, CardHeader, Box, Typography, TextField, Button, IconButton,
  Dialog, DialogTitle, DialogContent, DialogActions, Table, TableHead, TableRow, TableCell, TableBody,
  Switch, FormControlLabel, LinearProgress,
} from '@mui/material'
import Add from '@mui/icons-material/Add'
import Edit from '@mui/icons-material/Edit'
import Delete from '@mui/icons-material/Delete'
import TravelExplore from '@mui/icons-material/TravelExplore'
import Save from '@mui/icons-material/Save'
import Router from '@mui/icons-material/Router'
import CloudDownload from '@mui/icons-material/CloudDownload'
import { api, ECHProfile } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function ProxiesPanel() {
  const { t } = useI18n()
  const [config, setConfig] = useState<{ server_host: string; server_auth: string }>({ server_host: '', server_auth: '' })
  const [echList, setEchList] = useState<ECHProfile[]>([])
  const [editEch, setEditEch] = useState<ECHProfile | null>(null)
  const [open, setOpen] = useState(false)
  const [testResult, setTestResult] = useState('')

  // ECH auto-discovery
  const [discoverDomain, setDiscoverDomain] = useState('')
  const [discoverDoH, setDiscoverDoH] = useState('https://dns.google/dns-query')
  const [discovering, setDiscovering] = useState(false)
  const [discoverResult, setDiscoverResult] = useState('')

  useEffect(() => {
    api.getConfig().then(c => setConfig({ server_host: c.server_host || '', server_auth: c.server_auth || '' }))
    fetchEchList()
  }, [])

  const fetchEchList = async () => {
    try {
      const data = await api.getECHProfiles()
      setEchList(data || [])
    } catch { setEchList([]) }
  }

  const saveEch = async () => {
    if (!editEch) return
    await api.upsertECHProfile(editEch)
    setOpen(false)
    fetchEchList()
  }

  const deleteEch = async (id: string) => {
    await api.deleteECHProfile(id)
    fetchEchList()
  }

  const handleDiscoverECH = async () => {
    if (!discoverDomain) return
    setDiscovering(true)
    setDiscoverResult('')
    try {
      const res = await api.fetchECH(discoverDomain, discoverDoH)
      setDiscoverResult(res.config || '')
    } catch (e) {
      setDiscoverResult('Failed: ' + (e instanceof Error ? e.message : 'unknown error'))
    } finally {
      setDiscovering(false)
    }
  }

  return (
    <>
      <Box display="flex" flexDirection="column" gap={2}>
        {/* Server Node */}
        <Card>
          <CardHeader avatar={<Router />} title={t('proxies.server_node')} titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" flexDirection="column" gap={2} maxWidth={500}>
              <TextField label={t('proxies.node_host')} value={config.server_host} size="small"
                onChange={e => setConfig({ ...config, server_host: e.target.value })} />
              <TextField label={t('proxies.auth_secret')} value={config.server_auth} size="small" type="password"
                onChange={e => setConfig({ ...config, server_auth: e.target.value })} />
              <Box display="flex" gap={1}>
                <Button variant="contained" startIcon={<Save />} onClick={() => {
                  api.updateServerConfig(config.server_host, config.server_auth)
                }}>
                  {t('proxies.save_node')}
                </Button>
                <Button variant="outlined" startIcon={<TravelExplore />} onClick={async () => {
                  setTestResult(t('proxies.testing'))
                  try {
                    const res = await api.getStatus()
                    setTestResult(res.proxy_running ? t('common.success') : 'Proxy not running')
                  } catch { setTestResult(t('common.failed')) }
                }}>
                  {t('proxies.test_conn')}
                </Button>
              </Box>
              {testResult && <Typography variant="caption" color="text.secondary">{testResult}</Typography>}
            </Box>
          </CardContent>
        </Card>

        {/* ECH Configs */}
        <Card>
          <CardHeader
            title={t('proxies.ech_management')}
            titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
            action={<Button size="small" startIcon={<Add />} variant="contained" onClick={() => { setEditEch({ id: '', name: '', config: '', discovery_domain: '', doh_upstream: '', auto_update: false }); setOpen(true) }}>
              {t('proxies.add_ech')}
            </Button>}
          />
          <CardContent sx={{ pt: 0 }}>
            {echList.length === 0 ? (
              <Typography variant="body2" color="text.secondary">{t('proxies.no_ech')}</Typography>
            ) : (
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>{t('common.name')}</TableCell>
                    <TableCell>{t('dns.sni_fake')}</TableCell>
                    <TableCell>Auto</TableCell>
                    <TableCell align="right">{t('common.status')}</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {echList.map(ech => (
                    <TableRow key={ech.id}>
                      <TableCell>{ech.name}</TableCell>
                      <TableCell>{ech.discovery_domain || '-'}</TableCell>
                      <TableCell><Switch size="small" checked={ech.auto_update} disabled /></TableCell>
                      <TableCell align="right">
                        <IconButton size="small" onClick={() => { setEditEch(ech); setOpen(true) }}>
                          <Edit fontSize="small" />
                        </IconButton>
                        <IconButton size="small" onClick={() => deleteEch(ech.id)}>
                          <Delete fontSize="small" />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        {/* ECH Auto-Discovery */}
        <Card>
          <CardHeader
            title="ECH Auto-Discovery"
            titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
            avatar={<CloudDownload />}
          />
          <CardContent sx={{ pt: 0 }}>
            <Box display="flex" flexDirection="column" gap={2} maxWidth={500}>
              <Typography variant="caption" color="text.secondary">
                Probe a domain via DoH to discover its ECH configuration.
              </Typography>
              <TextField label="Domain" value={discoverDomain} size="small" fullWidth
                onChange={e => setDiscoverDomain(e.target.value)}
                placeholder="example.com" />
              <TextField label="DoH URL" value={discoverDoH} size="small" fullWidth
                onChange={e => setDiscoverDoH(e.target.value)} />
              <Box>
                <Button variant="outlined" startIcon={<CloudDownload />} onClick={handleDiscoverECH}
                  disabled={discovering || !discoverDomain}>
                  {t('proxies.probe_ech')}
                </Button>
                {discovering && <LinearProgress sx={{ mt: 1 }} />}
              </Box>
              {discoverResult && (
                <TextField label="Discovered ECH Config" value={discoverResult} size="small" fullWidth
                  multiline rows={6} slotProps={{ input: { readOnly: true } }} />
              )}
            </Box>
          </CardContent>
        </Card>
      </Box>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editEch?.id ? t('proxies.edit_ech') : t('proxies.add_ech')}</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <TextField label={t('common.name')} value={editEch?.name || ''} size="small" fullWidth
              onChange={e => setEditEch(prev => prev ? { ...prev, name: e.target.value } : null)} />
            <TextField label="Discovery Domain" value={editEch?.discovery_domain || ''} size="small" fullWidth
              onChange={e => setEditEch(prev => prev ? { ...prev, discovery_domain: e.target.value } : null)} />
            <TextField label="DoH Upstream" value={editEch?.doh_upstream || ''} size="small" fullWidth
              onChange={e => setEditEch(prev => prev ? { ...prev, doh_upstream: e.target.value } : null)} />
            <TextField label="ECH Config (JSON)" value={editEch?.config || ''} size="small" fullWidth multiline rows={6}
              onChange={e => setEditEch(prev => prev ? { ...prev, config: e.target.value } : null)} />
            <FormControlLabel
              control={<Switch checked={editEch?.auto_update ?? false}
                onChange={e => setEditEch(prev => prev ? { ...prev, auto_update: e.target.checked } : null)} />}
              label="Auto Update"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={saveEch}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
