import { Card, CardContent, CardHeader, Box, Typography } from '@mui/material'
import ArrowDownward from '@mui/icons-material/ArrowDownward'
import ArrowUpward from '@mui/icons-material/ArrowUpward'
import { StatsResponse } from '../api'
import { useI18n } from '../i18n/I18nContext'

interface Props {
  stats: StatsResponse
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

export default function TrafficStats({ stats }: Props) {
  const { t } = useI18n()

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title={t('dashboard.traffic')}
        titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
      />
      <CardContent sx={{ pt: 0 }}>
        <Box display="flex" gap={3}>
          <Box display="flex" alignItems="center" gap={1}>
            <Box
              sx={{
                width: 40,
                height: 40,
                borderRadius: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                bgcolor: 'rgba(46, 160, 67, 0.12)',
              }}
            >
              <ArrowDownward sx={{ color: 'success.main', fontSize: 22 }} />
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary" display="block">
                {t('dashboard.download')}
              </Typography>
              <Typography variant="body1" fontWeight={600}>
                {formatBytes(stats.bytes_down)}
              </Typography>
            </Box>
          </Box>

          <Box display="flex" alignItems="center" gap={1}>
            <Box
              sx={{
                width: 40,
                height: 40,
                borderRadius: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                bgcolor: 'rgba(25, 118, 210, 0.12)',
              }}
            >
              <ArrowUpward sx={{ color: 'primary.main', fontSize: 22 }} />
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary" display="block">
                {t('dashboard.upload')}
              </Typography>
              <Typography variant="body1" fontWeight={600}>
                {formatBytes(stats.bytes_up)}
              </Typography>
            </Box>
          </Box>
        </Box>
      </CardContent>
    </Card>
  )
}
