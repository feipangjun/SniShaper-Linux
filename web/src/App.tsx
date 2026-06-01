import { useState } from 'react'
import { ThemeProvider, CssBaseline, Container, AppBar, Toolbar, Typography, Box, IconButton, Tooltip, Tabs, Tab } from '@mui/material'
import Science from '@mui/icons-material/Science'
import Language from '@mui/icons-material/Language'
import Settings from '@mui/icons-material/Settings'
import Router from '@mui/icons-material/Router'
import Dns from '@mui/icons-material/Dns'
import Info from '@mui/icons-material/Info'
import Dashboard from '@mui/icons-material/Dashboard'
import { I18nProvider, useI18n } from './i18n/I18nContext'
import theme from './theme'
import DashboardPanel from './components/Dashboard'
import RulesPanel from './components/RulesPanel'
import SettingsPanel from './components/SettingsPanel'
import ProxiesPanel from './components/ProxiesPanel'
import RoutingPanel from './components/RoutingPanel'
import DNSPanel from './components/DNSPanel'
import AboutPanel from './components/AboutPanel'

const tabs = [
  { key: 'dashboard', icon: <Dashboard />, label: 'nav.dashboard' },
  { key: 'proxies', icon: <Router />, label: 'nav.proxies' },
  { key: 'rules', icon: <Science />, label: 'nav.rules' },
  { key: 'routing', icon: <Router />, label: 'nav.routing' },
  { key: 'dns', icon: <Dns />, label: 'nav.dns' },
  { key: 'settings', icon: <Settings />, label: 'nav.settings' },
  { key: 'about', icon: <Info />, label: 'nav.about' },
]

function AppInner() {
  const { t, lang, setLang } = useI18n()
  const [activeTab, setActiveTab] = useState(0)

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <AppBar position="static" sx={{ bgcolor: 'rgba(10, 25, 41, 0.95)', backdropFilter: 'blur(10px)', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
        <Toolbar variant="dense" sx={{ minHeight: 48 }}>
          <Science sx={{ mr: 1, color: 'primary.main', fontSize: 20 }} />
          <Typography variant="subtitle1" sx={{ fontWeight: 600, letterSpacing: 1, mr: 3 }}>
            SniShaper
            <Typography component="span" variant="caption" sx={{ ml: 1, opacity: 0.6 }}>
              Linux
            </Typography>
          </Typography>
          <Tabs
            value={activeTab}
            onChange={(_, v) => setActiveTab(v)}
            textColor="inherit"
            sx={{ flexGrow: 1, '& .MuiTab-root': { minHeight: 48, py: 0, textTransform: 'none', fontSize: '0.82rem' } }}
          >
            {tabs.map(tab => (
              <Tab key={tab.key} icon={tab.icon} label={t(tab.label)} iconPosition="start" />
            ))}
          </Tabs>
          <Tooltip title={lang === 'zh' ? 'Switch Language' : '切换语言'}>
            <IconButton color="inherit" size="small" onClick={() => {
              const langs: Array<'zh' | 'en' | 'ru'> = ['zh', 'en', 'ru']
              const idx = langs.indexOf(lang)
              setLang(langs[(idx + 1) % langs.length])
            }}>
              <Language fontSize="small" />
            </IconButton>
          </Tooltip>
        </Toolbar>
      </AppBar>
      <Container maxWidth="xl" sx={{ py: 2 }}>
        {activeTab === 0 && <DashboardPanel />}
        {activeTab === 1 && <ProxiesPanel />}
        {activeTab === 2 && <RulesPanel />}
        {activeTab === 3 && <RoutingPanel />}
        {activeTab === 4 && <DNSPanel />}
        {activeTab === 5 && <SettingsPanel />}
        {activeTab === 6 && <AboutPanel />}
      </Container>
    </Box>
  )
}

function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <I18nProvider>
        <AppInner />
      </I18nProvider>
    </ThemeProvider>
  )
}

export default App
