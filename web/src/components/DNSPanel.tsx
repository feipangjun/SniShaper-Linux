import { useState, useEffect } from 'react'
import {
  Card, CardContent, CardHeader, Box, Typography, Button, TextField, IconButton, Switch,
  Dialog, DialogTitle, DialogContent, DialogActions, Table, TableHead, TableRow, TableCell, TableBody, FormControlLabel,
} from '@mui/material'
import Add from '@mui/icons-material/Add'
import Edit from '@mui/icons-material/Edit'
import Delete from '@mui/icons-material/Delete'
import PlayArrow from '@mui/icons-material/PlayArrow'
import KeyboardArrowUp from '@mui/icons-material/KeyboardArrowUp'
import KeyboardArrowDown from '@mui/icons-material/KeyboardArrowDown'
import { api, DNSNode } from '../api'
import { useI18n } from '../i18n/I18nContext'

export default function DNSPanel() {
  const { t } = useI18n()
  const [nodes, setNodes] = useState<DNSNode[]>([])
  const [editNode, setEditNode] = useState<DNSNode | null>(null)
  const [open, setOpen] = useState(false)
  const [testResults, setTestResults] = useState<Record<string, { ips: string[]; status: string } | string>>({})
  const [testingId, setTestingId] = useState<string | null>(null)

  useEffect(() => { fetchNodes() }, [])

  const fetchNodes = async () => {
    try {
      const data = await api.getDNSNodes()
      setNodes(data || [])
    } catch { setNodes([]) }
  }

  const saveNode = async () => {
    if (!editNode) return
    if (editNode.id) {
      await api.updateDNSNode(editNode)
    } else {
      await api.addDNSNode(editNode)
    }
    setOpen(false)
    fetchNodes()
  }

  const deleteNode = async (id: string) => {
    await api.deleteDNSNode(id)
    fetchNodes()
  }

  const moveNode = async (index: number, direction: -1 | 1) => {
    const target = index + direction
    if (target < 0 || target >= nodes.length) return
    const nodeId = nodes[index].id
    try {
      await api.setDNSNodePriority(nodeId, target)
      fetchNodes()
    } catch {}
  }

  const testNode = async (id: string) => {
    setTestingId(id)
    try {
      const res = await api.testDNSNode(id)
      setTestResults(prev => ({ ...prev, [id]: { ips: res.ips, status: 'ok' } }))
    } catch (err: any) {
      setTestResults(prev => ({ ...prev, [id]: err?.message || 'test failed' }))
    } finally {
      setTestingId(null)
    }
  }

  const testAllNodes = async () => {
    for (const n of nodes) {
      await testNode(n.id)
    }
  }

  return (
    <>
      <Card>
        <CardHeader
          title={t('dns.title')}
          titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }}
          action={
            <Box display="flex" gap={1}>
              <Button size="small" startIcon={<PlayArrow />} variant="outlined" onClick={testAllNodes}>
                {t('dns.test_all')}
              </Button>
              <Button size="small" startIcon={<Add />} variant="contained" onClick={() => {
                setEditNode({
                  id: '', name: '', url: '', sni: '', ips: [],
                  ech_enabled: false, ech_profile_id: '', ech_auto_update: false,
                  quic: false, cert_verify: true, enabled: true,
                })
                setOpen(true)
              }}>
                {t('dns.add_node')}
              </Button>
            </Box>
          }
        />
        <CardContent sx={{ pt: 0 }}>
          {nodes.length === 0 ? (
            <Typography variant="body2" color="text.secondary">{t('dns.no_nodes')}</Typography>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell sx={{ width: 80 }}>#</TableCell>
                  <TableCell>{t('dns.node_name')}</TableCell>
                  <TableCell>{t('dns.doh_url')}</TableCell>
                  <TableCell>{t('dns.sni_fake')}</TableCell>
                  <TableCell>{t('dns.bootstrap_ips')}</TableCell>
                  <TableCell>ECH</TableCell>
                  <TableCell align="right">{t('common.status')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {nodes.map((node, idx) => (
                  <TableRow key={node.id}>
                    <TableCell>
                      <Box display="flex" alignItems="center">
                        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5, minWidth: 16 }}>
                          {idx + 1}
                        </Typography>
                        <IconButton size="small" disabled={idx === 0} onClick={() => moveNode(idx, -1)}>
                          <KeyboardArrowUp fontSize="small" />
                        </IconButton>
                        <IconButton size="small" disabled={idx === nodes.length - 1} onClick={() => moveNode(idx, 1)}>
                          <KeyboardArrowDown fontSize="small" />
                        </IconButton>
                      </Box>
                    </TableCell>
                    <TableCell>{node.name}</TableCell>
                    <TableCell sx={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis' }}>{node.url}</TableCell>
                    <TableCell>{node.sni || '-'}</TableCell>
                    <TableCell>{(node.ips || []).join(', ') || '-'}</TableCell>
                    <TableCell>
                      <Switch size="small" checked={node.ech_enabled} disabled />
                    </TableCell>
                    <TableCell align="right">
                      <Box display="flex" alignItems="center" gap={0.5}>
                        {testingId === node.id && <Typography variant="caption" color="text.secondary">...</Typography>}
                        {testResults[node.id] && typeof testResults[node.id] === 'object' && (
                          <Typography variant="caption" color="success.light" sx={{ mr: 0.5 }}>
                            {(testResults[node.id] as any).ips?.join(', ')}
                          </Typography>
                        )}
                        {testResults[node.id] && typeof testResults[node.id] === 'string' && (
                          <Typography variant="caption" color="error.light" sx={{ mr: 0.5, maxWidth: 100, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            {testResults[node.id] as string}
                          </Typography>
                        )}
                        <IconButton size="small" disabled={testingId === node.id} onClick={() => testNode(node.id)}>
                          <PlayArrow fontSize="small" />
                        </IconButton>
                      </Box>
                      <IconButton size="small" onClick={() => { setEditNode(node); setOpen(true) }}>
                        <Edit fontSize="small" />
                      </IconButton>
                      <IconButton size="small" onClick={() => deleteNode(node.id)}>
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

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editNode?.id ? t('dns.edit_node') : t('dns.add_node')}</DialogTitle>
        <DialogContent>
          <Box display="flex" flexDirection="column" gap={2} pt={1}>
            <TextField label={t('dns.node_name')} value={editNode?.name || ''} size="small" fullWidth
              onChange={e => setEditNode(prev => prev ? { ...prev, name: e.target.value } : null)} />
            <TextField label={t('dns.doh_url')} value={editNode?.url || ''} size="small" fullWidth
              onChange={e => setEditNode(prev => prev ? { ...prev, url: e.target.value } : null)} />
            <TextField label={t('dns.sni_fake')} value={editNode?.sni || ''} size="small" fullWidth
              onChange={e => setEditNode(prev => prev ? { ...prev, sni: e.target.value } : null)} />
            <TextField label={t('dns.bootstrap_ips') + ' (comma-separated)'} value={(editNode?.ips || []).join(',')} size="small" fullWidth
              onChange={e => setEditNode(prev => prev ? { ...prev, ips: e.target.value.split(',').map(s => s.trim()).filter(Boolean) } : null)} />
            <FormControlLabel
              control={<Switch checked={editNode?.ech_enabled ?? false}
                onChange={e => setEditNode(prev => prev ? { ...prev, ech_enabled: e.target.checked } : null)} />}
              label="ECH"
            />
            <FormControlLabel
              control={<Switch checked={editNode?.quic ?? false}
                onChange={e => setEditNode(prev => prev ? { ...prev, quic: e.target.checked } : null)} />}
              label="QUIC"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={saveNode}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
