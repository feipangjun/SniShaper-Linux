import { useState, useEffect } from 'react'
import { Card, CardContent, Box, Typography, Chip, Link } from '@mui/material'
import Science from '@mui/icons-material/Science'
import Shield from '@mui/icons-material/Shield'
import Speed from '@mui/icons-material/Speed'
import Code from '@mui/icons-material/Code'
import { api } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function AboutPanel() {
  const { t } = useI18n()
  const [version, setVersion] = useState('')

  useEffect(() => {
    api.getConfig().then(c => setVersion(c.version || '')).catch(() => setVersion('unknown'))
  }, [])

  const features = [
    { icon: <Shield />, title: t('about.feature_ech'), desc: t('about.feature_ech_desc') },
    { icon: <Speed />, title: t('about.feature_fast'), desc: t('about.feature_fast_desc') },
    { icon: <Code />, title: t('about.feature_open'), desc: t('about.feature_open_desc') },
  ]

  return (
    <Card>
      <CardContent>
        <Box display="flex" flexDirection="column" alignItems="center" gap={3} py={4}>
          <Science sx={{ fontSize: 64, color: 'primary.main' }} />
          <Typography variant="h5" fontWeight={700}>SniShaper</Typography>
          <Chip label={`${t('about.version')}: ${version}`} variant="outlined" size="small" />

          <Typography variant="body2" color="text.secondary" textAlign="center" maxWidth={600}>
            {t('about.description')}
          </Typography>

          <Box>
            <Typography variant="subtitle2" fontWeight={600} gutterBottom textAlign="center">
              {t('about.features')}
            </Typography>
            <Box display="flex" gap={2} flexWrap="wrap" justifyContent="center">
              {features.map((f, i) => (
                <Box key={i} display="flex" alignItems="center" gap={1} sx={{
                  p: 2, borderRadius: 2, border: '1px solid', borderColor: 'divider', minWidth: 200,
                }}>
                  {f.icon}
                  <Box>
                    <Typography variant="body2" fontWeight={600}>{f.title}</Typography>
                    <Typography variant="caption" color="text.secondary">{f.desc}</Typography>
                  </Box>
                </Box>
              ))}
            </Box>
          </Box>

          <Typography variant="caption" color="text.secondary">
            {t('about.community')}:{' '}
            <Link href="https://github.com/anomalyco/SniShaper" target="_blank" rel="noopener">GitHub</Link>
          </Typography>
        </Box>
      </CardContent>
    </Card>
  )
}
