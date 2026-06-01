const API_BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export interface StatusResponse {
  version: string
  proxy_running: boolean
  listen_addr: string
  proxy_mode: string
  tun: TUNStatus
  cert_installed: boolean
  socks5_enabled: boolean
}

export interface StatsResponse {
  bytes_down: number
  bytes_up: number
}

export interface SiteGroup {
  id: string
  name: string
  website?: string
  domains: string[]
  mode: string
  upstream: string
  upstreams?: string[]
  sni_fake: string
  enabled: boolean
  ech_enabled: boolean
  ech_profile_id?: string
  ech_domain?: string
  use_cf_pool: boolean
  dns_mode?: string
  connect_policy?: string
  sni_policy?: string
}

export interface Upstream {
  id: string
  name: string
  address: string
  enabled: boolean
}

export interface CertStatus {
  Installed: boolean
  Platform: string
  CertPath: string
  InstallHelp: string
}

export interface TUNStatus {
  supported: boolean
  running: boolean
  enabled: boolean
  driver?: string
  message?: string
  mtu?: number
  dns_hijack?: boolean
  auto_route?: boolean
  strict_route?: boolean
}

export interface TUNConfig {
  mtu?: number
  dns_hijack?: boolean
  auto_route?: boolean
  strict_route?: boolean
}

export interface ConfigResponse {
  listen_port: string
  socks5_port?: string
  socks5_enabled: boolean
  proxy_mode: string
  tun: TUNConfig
  auto_routing: AutoRoutingConfig
  cloudflare: CloudflareConfig
  server_host: string
  server_auth: string
  language: string
  theme: string
  version?: string
}

export interface DNSNode {
  id: string
  name: string
  url: string
  sni?: string
  ips?: string[]
  ech_enabled: boolean
  ech_profile_id?: string
  ech_auto_update: boolean
  quic: boolean
  cert_verify: Record<string, unknown> | boolean
  enabled: boolean
}

export interface ECHProfile {
  id: string
  name: string
  config: string
  discovery_domain?: string
  doh_upstream?: string
  auto_update: boolean
}

export interface AutoRoutingConfig {
  mode: string
  gfwlist_url?: string
  last_update?: string
}

export interface GFWListStatus {
  enabled: boolean
  mode: string
  domain_count: number
  last_update: string
  gfwlist_url: string
}

export interface CloudflareConfig {
  preferred_ips: string[]
  auto_update: boolean
  api_key: string
}

export interface CFIPStats {
  ip: string
  latency: string
  failures: number
  last_check: string
}

export interface ProxyDiagnostics {
  accepted: number
  requests: number
  connects: number
  recent_ingress: string[]
  listen_addr: string
  proxy_running: boolean
}

export const api = {
  getStatus: () => request<StatusResponse>('/status'),
  startProxy: () => request<{ status: string }>('/proxy/start', { method: 'POST' }),
  stopProxy: () => request<{ status: string }>('/proxy/stop', { method: 'POST' }),
  getStats: () => request<StatsResponse>('/proxy/stats'),
  getDiagnostics: () => request<ProxyDiagnostics>('/proxy/diagnostics'),
  selfCheck: () => request<{ status: string; message: string }>('/proxy/self-check', { method: 'POST' }),

  getConfig: () => request<ConfigResponse>('/config'),
  updateConfig: (cfg: Partial<ConfigResponse>) => request<{ status: string }>('/config', { method: 'POST', body: JSON.stringify(cfg) }),

  getCertStatus: () => request<CertStatus>('/cert/status'),
  installCert: (password?: string) => request<{ status: string }>('/cert/install', { method: 'POST', body: JSON.stringify({ password }) }),
  uninstallCert: (password?: string) => request<{ status: string }>('/cert/uninstall', { method: 'POST', body: JSON.stringify({ password }) }),
  regenerateCert: (password?: string) => request<{ status: string }>('/cert/regenerate', { method: 'POST', body: JSON.stringify({ password }) }),

  getRules: () => request<SiteGroup[]>('/rules'),
  addRule: (sg: Partial<SiteGroup>) => request<{ status: string }>('/rules', { method: 'POST', body: JSON.stringify(sg) }),
  updateRule: (sg: SiteGroup) => request<{ status: string }>('/rules', { method: 'PUT', body: JSON.stringify(sg) }),
  deleteRule: (id: string) => request<{ status: string }>(`/rules?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getRuleHits: () => request<Record<string, number>>('/rules/hits'),

  getUpstreams: () => request<Upstream[]>('/upstreams'),
  addUpstream: (u: Partial<Upstream>) => request<{ status: string }>('/upstreams', { method: 'POST', body: JSON.stringify(u) }),
  updateUpstream: (u: Upstream) => request<{ status: string }>('/upstreams', { method: 'PUT', body: JSON.stringify(u) }),
  deleteUpstream: (id: string) => request<{ status: string }>(`/upstreams?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),

  getTUNStatus: () => request<TUNStatus>('/tun/status'),
  startTUN: () => request<{ status: string }>('/tun/start', { method: 'POST' }),
  stopTUN: () => request<{ status: string }>('/tun/stop', { method: 'POST' }),

  getLogs: (limit = 200) => request<{ logs: string }>(`/logs?limit=${limit}`),
  clearLogs: () => request<{ status: string }>('/logs/clear', { method: 'POST' }),
  getLogCapture: () => request<{ capture_enabled: boolean }>('/log/capture'),
  setLogCapture: (enabled: boolean) => request<{ status: string }>('/log/capture', { method: 'POST', body: JSON.stringify({ enabled }) }),

  getDNSNodes: () => request<DNSNode[]>('/dns/nodes'),
  addDNSNode: (n: Partial<DNSNode>) => request<{ status: string }>('/dns/nodes', { method: 'POST', body: JSON.stringify(n) }),
  updateDNSNode: (n: DNSNode) => request<{ status: string }>('/dns/nodes', { method: 'PUT', body: JSON.stringify(n) }),
  deleteDNSNode: (id: string) => request<{ status: string }>(`/dns/nodes?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  testDNSNode: (id: string) => request<{ ips: string[]; status: string }>('/dns/test', { method: 'POST', body: JSON.stringify({ id }) }),
  setDNSNodePriority: (id: string, targetIndex: number) =>
    request<{ status: string }>('/dns/priority', { method: 'POST', body: JSON.stringify({ id, target_index: targetIndex }) }),

  getECHProfiles: () => request<ECHProfile[]>('/ech/profiles'),
  upsertECHProfile: (p: Partial<ECHProfile>) => request<{ status: string }>('/ech/profiles', { method: 'POST', body: JSON.stringify(p) }),
  deleteECHProfile: (id: string) => request<{ status: string }>(`/ech/profiles?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  fetchECH: (domain: string, dohURL: string) => request<{ config: string; status: string }>('/ech/fetch', { method: 'POST', body: JSON.stringify({ domain, doh_url: dohURL }) }),

  getCFConfig: () => request<CloudflareConfig>('/cf/config'),
  updateCFConfig: (cfg: CloudflareConfig) => request<{ status: string }>('/cf/config', { method: 'POST', body: JSON.stringify(cfg) }),
  fetchCFIPs: () => request<{ ips: string[]; count: number }>('/cf/fetch', { method: 'POST' }),
  triggerCFHealthCheck: () => request<{ status: string }>('/cf/health-check', { method: 'POST' }),
  getCFStats: () => request<CFIPStats[]>('/cf/stats'),

  getAutoRouteConfig: () => request<AutoRoutingConfig>('/routing/config'),
  updateAutoRouteConfig: (mode: string) => request<{ status: string }>('/routing/config', { method: 'POST', body: JSON.stringify({ mode }) }),
  refreshGFWList: () => request<{ count: number; status: string }>('/routing/gfwlist/refresh', { method: 'POST' }),
  getAutoRouteStatus: () => request<GFWListStatus>('/routing/status'),

  getServerConfig: () => request<{ host: string; auth: string }>('/server'),
  updateServerConfig: (host: string, auth: string) => request<{ status: string }>('/server', { method: 'POST', body: JSON.stringify({ host, auth }) }),

  exportConfig: () => request<{ config: string }>('/config/export', { method: 'POST' }),
  importConfig: (config: string) =>
    request<{ total: number; added: number; overwritten: number; skipped: number }>('/config/import', { method: 'POST', body: JSON.stringify({ config }) }),
}
