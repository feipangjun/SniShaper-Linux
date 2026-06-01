import { useState, useEffect, useRef, useCallback } from 'react'
import { Card, CardContent, CardHeader, Box, Typography, IconButton, TextField, Tooltip, Button, Switch, FormControlLabel } from '@mui/material'
import Clear from '@mui/icons-material/Clear'
import Download from '@mui/icons-material/Download'
import Pause from '@mui/icons-material/Pause'
import PlayArrow from '@mui/icons-material/PlayArrow'
import FilterAlt from '@mui/icons-material/FilterAlt'
import { api } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function LogViewer() {
  const { t } = useI18n()
  const [logs, setLogs] = useState<string[]>([])
  const [autoScroll, setAutoScroll] = useState(true)
  const [captureEnabled, setCaptureEnabled] = useState(true)
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const initialized = useRef(false)
  const logEndRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const fetchLogs = useCallback(async () => {
    if (paused) return
    try {
      const res = await api.getLogs(500)
      const lines = res.logs ? res.logs.split('\n').filter(Boolean) : []
      setLogs(lines)
    } catch {}
  }, [paused])

  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true
      api.getLogCapture().then(res => setCaptureEnabled(res.capture_enabled)).catch(() => {})
    }
    fetchLogs()
    const timer = setInterval(fetchLogs, 2000)
    return () => clearInterval(timer)
  }, [fetchLogs])

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  const handleScroll = () => {
    if (!containerRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 50)
  }

  const filtered = filter
    ? logs.filter(l => l.toLowerCase().includes(filter.toLowerCase()))
    : logs

  const handleClear = async () => {
    try {
      await api.clearLogs()
      setLogs([])
    } catch {}
  }

  const handleExport = () => {
    const blob = new Blob([logs.join('\n')], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `snishaper-logs-${new Date().toISOString().slice(0, 19)}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const lineCount = filtered.length

  return (
    <Card>
      <CardHeader
        title={
          <Box display="flex" alignItems="center" gap={1}>
            <Typography variant="subtitle1" fontWeight={600}>{t('dashboard.logs')}</Typography>
            {lineCount > 0 && (
              <Typography variant="caption" color="text.secondary">
                {lineCount} lines
              </Typography>
            )}
          </Box>
        }
        action={
          <Box display="flex" gap={0.5} alignItems="center">
            <FormControlLabel
              control={
                <Switch
                  size="small"
                  checked={captureEnabled}
                  onChange={async () => {
                    const next = !captureEnabled
                    setCaptureEnabled(next)
                    await api.setLogCapture(next)
                  }}
                />
              }
              label="Capture"
              sx={{ mr: 0, '& .MuiTypography-root': { fontSize: '0.75rem' } }}
            />
            <Tooltip title={paused ? 'Resume' : 'Pause'}>
              <IconButton size="small" onClick={() => setPaused(!paused)}>
                {paused ? <PlayArrow fontSize="small" /> : <Pause fontSize="small" />}
              </IconButton>
            </Tooltip>
            <Tooltip title={autoScroll ? t('dashboard.pause_scroll') : t('dashboard.resume_scroll')}>
              <IconButton size="small" onClick={() => setAutoScroll(!autoScroll)}>
                <FilterAlt fontSize="small" sx={{ opacity: autoScroll ? 1 : 0.4 }} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('dashboard.export_logs')}>
              <IconButton size="small" onClick={handleExport}>
                <Download fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('dashboard.clear_logs')}>
              <IconButton size="small" onClick={handleClear}>
                <Clear fontSize="small" />
              </IconButton>
            </Tooltip>
            <TextField
              size="small"
              placeholder={t('dashboard.filter_placeholder')}
              value={filter}
              onChange={e => setFilter(e.target.value)}
              sx={{ ml: 1, width: 140 }}
              slotProps={{
                input: {
                  sx: { fontSize: '0.8rem', py: 0.3 },
                },
              }}
            />
          </Box>
        }
      />
      <CardContent sx={{ pt: 0 }}>
        <Box
          ref={containerRef}
          onScroll={handleScroll}
          sx={{
            height: 300,
            overflow: 'auto',
            bgcolor: 'rgba(0,0,0,0.3)',
            borderRadius: 1,
            p: 1,
            fontFamily: '"Fira Code", "Cascadia Code", "Consolas", monospace',
            fontSize: '0.75rem',
            lineHeight: 1.6,
            opacity: captureEnabled ? 1 : 0.5,
            transition: 'opacity 0.3s',
            '&::-webkit-scrollbar': { width: 6 },
            '&::-webkit-scrollbar-track': { bgcolor: 'transparent' },
            '&::-webkit-scrollbar-thumb': { bgcolor: 'rgba(255,255,255,0.15)', borderRadius: 3 },
          }}
        >
          {filtered.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ p: 2, textAlign: 'center' }}>
              {t('dashboard.no_logs')}
            </Typography>
          ) : (
            filtered.map((line, i) => (
              <Box
                key={i}
                component="div"
                sx={{
                  color: line.includes('[core]') ? 'info.light' :
                         line.includes('error') || line.includes('Error') ? 'error.light' :
                         line.includes('warn') ? 'warning.light' :
                         'text.primary',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {line}
              </Box>
            ))
          )}
          <div ref={logEndRef} />
        </Box>
      </CardContent>
    </Card>
  )
}
