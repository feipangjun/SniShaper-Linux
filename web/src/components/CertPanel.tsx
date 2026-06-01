import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, Box, Typography, Button, Chip, CircularProgress, TextField } from '@mui/material'
import Security from '@mui/icons-material/Security'
import CheckCircle from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import Refresh from '@mui/icons-material/Refresh'
import { api, CertStatus } from '../api'
import { useI18n } from '../i18n/I18nContext'

interface Props {
  onError: (msg: string) => void
}

export default function CertPanel({ onError }: Props) {
  const { t } = useI18n()
  const [cert, setCert] = useState<CertStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [passwordDialog, setPasswordDialog] = useState<{ action: 'install' | 'uninstall' | 'regenerate'; password: string } | null>(null)

  const refresh = async () => {
    try {
      const s = await api.getCertStatus()
      setCert(s)
    } catch (e) {
      onError(e instanceof Error ? e.message : t('dashboard.refresh_failed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { refresh() }, [])

  const runWithPassword = async (password: string) => {
    const action = passwordDialog?.action
    setPasswordDialog(null)
    if (!action) return
    setActionLoading(true)
    try {
      const pwd = password || undefined
      if (action === 'install') await api.installCert(pwd)
      else if (action === 'uninstall') await api.uninstallCert(pwd)
      else await api.regenerateCert(pwd)
      await refresh()
    } catch (e) {
      onError(e instanceof Error ? e.message : t('dashboard.operation_failed'))
    } finally {
      setActionLoading(false)
    }
  }

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title={t('dashboard.certificate')}
        titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
        action={
          <Button size="small" onClick={refresh} startIcon={<Refresh />}>
            {t('dashboard.refresh')}
          </Button>
        }
      />
      <CardContent sx={{ pt: 0 }}>
        {loading ? (
          <CircularProgress size={24} />
        ) : (
          <>
            <Box display="flex" alignItems="center" gap={1} mb={2}>
              <Security sx={{ color: cert?.Installed ? 'success.main' : 'warning.main', fontSize: 32 }} />
              <Box>
                <Typography variant="h6" fontWeight={700}>
                  {cert?.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {cert?.Platform || 'linux'}
                </Typography>
              </Box>
            </Box>

            <Box display="flex" gap={1} mb={1.5}>
              {cert?.Installed ? (
                <Chip icon={<CheckCircle />} label={t('dashboard.trusted')} color="success" size="small" />
              ) : (
                <Chip icon={<ErrorIcon />} label={t('dashboard.untrusted')} color="warning" size="small" />
              )}
            </Box>

            <Box display="flex" gap={1} flexWrap="wrap">
              {cert?.Installed ? (
                <>
                  <Button
                    variant="outlined"
                    size="small"
                    color="warning"
                    disabled={actionLoading}
                    onClick={() => setPasswordDialog({ action: 'uninstall', password: '' })}
                  >
                    {t('dashboard.uninstall')}
                  </Button>
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={actionLoading}
                    onClick={() => setPasswordDialog({ action: 'regenerate', password: '' })}
                  >
                    {t('dashboard.regenerate')}
                  </Button>
                </>
              ) : (
                <Button
                  variant="contained"
                  size="small"
                  color="primary"
                  disabled={actionLoading}
                  onClick={() => setPasswordDialog({ action: 'install', password: '' })}
                >
                  {t('dashboard.install_ca')}
                </Button>
              )}
            </Box>

            {passwordDialog && (
              <Box mt={2} display="flex" gap={1} alignItems="center">
                <TextField
                  size="small"
                  type="password"
                  placeholder="sudo password (optional)"
                  value={passwordDialog.password}
                  onChange={e => setPasswordDialog({ ...passwordDialog, password: e.target.value })}
                  onKeyDown={e => e.key === 'Enter' && runWithPassword(passwordDialog.password)}
                  sx={{ flex: 1 }}
                />
                <Button size="small" variant="contained" onClick={() => runWithPassword(passwordDialog.password)}>
                  OK
                </Button>
                <Button size="small" onClick={() => setPasswordDialog(null)}>
                  {t('common.cancel')}
                </Button>
              </Box>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
