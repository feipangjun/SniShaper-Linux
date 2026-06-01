import { useState, useEffect } from 'react'
import {
  Card, CardContent, CardHeader, Box, Typography, Chip, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, Paper, TextField, InputAdornment, Collapse, IconButton,
  Button, Dialog, DialogTitle, DialogContent, DialogActions, Select, MenuItem, FormControl, InputLabel, Switch, FormControlLabel,
} from '@mui/material'
import Search from '@mui/icons-material/Search'
import ExpandMore from '@mui/icons-material/ExpandMore'
import ExpandLess from '@mui/icons-material/ExpandLess'
import CheckCircle from '@mui/icons-material/CheckCircle'
import Cancel from '@mui/icons-material/Cancel'
import Add from '@mui/icons-material/Add'
import Edit from '@mui/icons-material/Edit'
import Delete from '@mui/icons-material/Delete'
import { api, SiteGroup } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function RulesPanel() {
  const { t } = useI18n()
  const [rules, setRules] = useState<SiteGroup[]>([])
  const [search, setSearch] = useState('')
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [editRule, setEditRule] = useState<SiteGroup | null>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => { fetchRules() }, [])

  const fetchRules = async () => {
    try {
      const data = await api.getRules()
      setRules(data || [])
    } catch { setRules([]) }
  }

  const saveRule = async () => {
    if (!editRule) return
    if (editRule.id) {
      await api.updateRule(editRule)
    } else {
      await api.addRule(editRule)
    }
    setOpen(false)
    fetchRules()
  }

  const deleteRule = async (id: string) => {
    await api.deleteRule(id)
    fetchRules()
  }

  const filtered = search
    ? rules.filter(r =>
        r.name.toLowerCase().includes(search.toLowerCase()) ||
        r.domains.some(d => d.toLowerCase().includes(search.toLowerCase()))
      )
    : rules

  const enabledCount = rules.filter(r => r.enabled).length

  const toggleExpand = (id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const emptyRule = (): SiteGroup => ({
    id: '', name: '', website: '', domains: [], mode: 'smart', upstream: '',
    sni_fake: '', enabled: true, ech_enabled: false, use_cf_pool: false,
  })

  return (
    <>
      <Card>
        <CardHeader
          title={
            <Box display="flex" alignItems="center" gap={1}>
              <Typography variant="subtitle1" fontWeight={600}>{t('dashboard.rules')}</Typography>
              <Chip label={`${enabledCount}/${rules.length}`} size="small" color="primary" variant="outlined" />
            </Box>
          }
          action={
            <Box display="flex" gap={1}>
              <TextField
                size="small"
                placeholder={t('dashboard.search_domains')}
                value={search}
                onChange={e => setSearch(e.target.value)}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search fontSize="small" />
                      </InputAdornment>
                    ),
                  },
                }}
                sx={{ width: 220 }}
              />
              <Button size="small" variant="contained" startIcon={<Add />} onClick={() => {
                setEditRule(emptyRule())
                setOpen(true)
              }}>
                {t('common.add')}
              </Button>
            </Box>
          }
        />
        <CardContent sx={{ pt: 0 }}>
          <TableContainer component={Paper} sx={{ maxHeight: 500, bgcolor: 'rgba(0,0,0,0.2)' }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}>{t('dashboard.status_col')}</TableCell>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}>{t('dashboard.name_col')}</TableCell>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}>{t('dashboard.mode_col')}</TableCell>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}>{t('dashboard.ech_col')}</TableCell>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}>{t('dashboard.domains_col')}</TableCell>
                  <TableCell sx={{ fontWeight: 700, bgcolor: 'rgba(0,0,0,0.4)' }}></TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filtered.slice(0, 200).map(sg => (
                  <>
                    <TableRow key={sg.id} hover>
                      <TableCell>
                        {sg.enabled
                          ? <CheckCircle sx={{ color: 'success.main', fontSize: 18 }} />
                          : <Cancel sx={{ color: 'action.disabled', fontSize: 18 }} />}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>{sg.name}</Typography>
                      </TableCell>
                      <TableCell>
                        <Chip label={sg.mode} size="small" variant="outlined" color="primary" />
                      </TableCell>
                      <TableCell>
                        {sg.ech_enabled
                          ? <Chip label="ECH" size="small" color="secondary" />
                          : <Typography variant="caption" color="text.secondary">—</Typography>}
                      </TableCell>
                      <TableCell>
                        <Typography variant="caption" color="text.secondary">
                          {t('dashboard.domains_count', { count: sg.domains.length })}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Box display="flex" gap={0.5}>
                          {sg.domains.length > 0 && (
                            <IconButton size="small" onClick={() => toggleExpand(sg.id)}>
                              {expanded[sg.id] ? <ExpandLess fontSize="small" /> : <ExpandMore fontSize="small" />}
                            </IconButton>
                          )}
                          <IconButton size="small" onClick={() => { setEditRule(sg); setOpen(true) }}>
                            <Edit fontSize="small" />
                          </IconButton>
                          <IconButton size="small" onClick={() => deleteRule(sg.id)}>
                            <Delete fontSize="small" />
                          </IconButton>
                        </Box>
                      </TableCell>
                    </TableRow>
                    <TableRow key={`${sg.id}-expand`}>
                      <TableCell colSpan={6} sx={{ py: 0 }}>
                        <Collapse in={expanded[sg.id]}>
                          <Box sx={{ py: 1, pl: 2 }}>
                            <Box display="flex" flexWrap="wrap" gap={0.5}>
                              {sg.domains.map(d => (
                                <Chip key={d} label={d} size="small" variant="outlined" sx={{ fontSize: '0.7rem' }} />
                              ))}
                            </Box>
                          </Box>
                        </Collapse>
                      </TableCell>
                    </TableRow>
                  </>
                ))}
                {filtered.length > 200 && (
                  <TableRow>
                    <TableCell colSpan={6} align="center">
                      <Typography variant="caption" color="text.secondary">
                        {t('dashboard.showing_rules', { shown: 200, total: filtered.length })}
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editRule?.id ? t('common.edit') : t('common.add')} Rule</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <TextField label={t('dashboard.name_col')} value={editRule?.name || ''} size="small" fullWidth
              onChange={e => setEditRule(prev => prev ? { ...prev, name: e.target.value } : null)} />
            <FormControl size="small" fullWidth>
              <InputLabel>{t('dashboard.mode_col')}</InputLabel>
              <Select value={editRule?.mode || 'smart'} label={t('dashboard.mode_col')}
                onChange={e => setEditRule(prev => prev ? { ...prev, mode: e.target.value } : null)}>
                <MenuItem value="smart">Smart</MenuItem>
                <MenuItem value="forward">Forward</MenuItem>
                <MenuItem value="direct">Direct</MenuItem>
                <MenuItem value="block">Block</MenuItem>
              </Select>
            </FormControl>
            <TextField label={t('dashboard.domains_col') + ' (逗号分隔)'} value={(editRule?.domains || []).join(',')} size="small" fullWidth
              onChange={e => setEditRule(prev => prev ? { ...prev, domains: e.target.value.split(',').map(s => s.trim()).filter(Boolean) } : null)} />
            <TextField label="Upstream" value={editRule?.upstream || ''} size="small" fullWidth
              onChange={e => setEditRule(prev => prev ? { ...prev, upstream: e.target.value } : null)} />
            <TextField label="SNI Fake" value={editRule?.sni_fake || ''} size="small" fullWidth
              onChange={e => setEditRule(prev => prev ? { ...prev, sni_fake: e.target.value } : null)} />
            <FormControlLabel
              control={<Switch checked={editRule?.enabled ?? true}
                onChange={e => setEditRule(prev => prev ? { ...prev, enabled: e.target.checked } : null)} />}
              label={t('common.enabled')}
            />
            <FormControlLabel
              control={<Switch checked={editRule?.ech_enabled ?? false}
                onChange={e => setEditRule(prev => prev ? { ...prev, ech_enabled: e.target.checked } : null)} />}
              label="ECH"
            />
            <FormControlLabel
              control={<Switch checked={editRule?.use_cf_pool ?? false}
                onChange={e => setEditRule(prev => prev ? { ...prev, use_cf_pool: e.target.checked } : null)} />}
              label="CF Pool"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={saveRule}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
