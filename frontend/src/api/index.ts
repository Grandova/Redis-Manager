import request from './request';
import {
  AdminUser,
  SystemInfo,
  PortCheckResult,
  DomainDNSResult,
  RedisDiscovery,
  HistoryMetrics,
  DBStat,
  KeyDetail,
  ConnectedClient,
  SlowLogItem,
  TLSStatusResponse,
  BackupItem,
  ConfigBackupItem,
  AuditLogItem,
  VPNGeoSummary,
} from '../types';

export const authApi = {
  login: (data: { username: string; password: string }): Promise<{ token: string; admin: AdminUser }> =>
    request.post('/auth/login', data),
  logout: (): Promise<void> => request.post('/auth/logout'),
  current: (): Promise<AdminUser> => request.get('/auth/current'),
  updatePassword: (data: { old_password: string; new_password: string }): Promise<void> =>
    request.put('/auth/password', data),
};

export const systemApi = {
  getInfo: (): Promise<SystemInfo> => request.get('/system/info'),
  checkPort: (port: number): Promise<PortCheckResult> => request.get(`/system/ports?port=${port}`),
  checkDNS: (domain: string): Promise<DomainDNSResult> => request.get(`/system/dns-check?domain=${domain}`),
};

export const redisApi = {
  getDiscovery: (): Promise<RedisDiscovery> => request.get('/redis/discovery'),
  install: (version: string): Promise<RedisDiscovery> => request.post('/redis/install', { version }),
  start: (service_name?: string): Promise<void> => request.post('/redis/start', { service_name }),
  stop: (service_name?: string): Promise<void> => request.post('/redis/stop', { service_name }),
  restart: (service_name?: string): Promise<void> => request.post('/redis/restart', { service_name }),
  reload: (service_name?: string): Promise<void> => request.post('/redis/reload', { service_name }),
  setAutoStart: (enable: boolean, service_name?: string): Promise<void> =>
    request.post('/redis/autostart', { enable, service_name }),
  uninstall: (confirm_code: string): Promise<void> => request.post('/redis/uninstall', { confirm_code }),
  getLogs: (lines = 200, service_name = 'redis-server', log_file = ''): Promise<{ lines: string[]; total: number }> =>
    request.get(`/redis/logs?lines=${lines}&service_name=${service_name}&log_file=${log_file}`),
};

export const monitorApi = {
  getInfo: (): Promise<Record<string, Record<string, string>>> => request.get('/redis/info'),
  getHistory: (range = '1h'): Promise<HistoryMetrics> => request.get(`/redis/metrics/history?range=${range}`),
};

export const configApi = {
  getConfig: (): Promise<{ config_path: string; items: Record<string, string> }> => request.get('/redis/config'),
  updateConfig: (data: Record<string, string>): Promise<void> => request.put('/redis/config', data),
  getRawConfig: (): Promise<{ config_path: string; content: string }> => request.get('/redis/config/raw'),
  updateRawConfig: (content: string, remark?: string): Promise<void> =>
    request.put('/redis/config/raw', { content, remark }),
  getBackups: (): Promise<ConfigBackupItem[]> => request.get('/redis/config/backups'),
  rollback: (backup_id: number): Promise<void> => request.post('/redis/config/rollback', { backup_id }),
};

export const dataApi = {
  getDBStats: (): Promise<DBStat[]> => request.get('/redis/data/db-stats'),
  scanKeys: (db: number, cursor: number, pattern: string, count: number, type?: string): Promise<any> =>
    request.get(`/redis/data/keys?db=${db}&cursor=${cursor}&pattern=${encodeURIComponent(pattern)}&count=${count}&type=${type || ''}`),
  getKey: (db: number, key: string): Promise<KeyDetail> =>
    request.get(`/redis/data/key?db=${db}&key=${encodeURIComponent(key)}`),
  saveKey: (db: number, data: any): Promise<void> => request.post(`/redis/data/key?db=${db}`, data),
  deleteKey: (db: number, key: string): Promise<void> =>
    request.delete(`/redis/data/key?db=${db}&key=${encodeURIComponent(key)}`),
  batchDeleteKeys: (db: number, keys: string[]): Promise<void> =>
    request.post(`/redis/data/keys/batch-delete?db=${db}`, { keys }),
  setTTL: (db: number, key: string, ttl: number): Promise<void> =>
    request.put(`/redis/data/key/ttl?db=${db}`, { key, ttl }),
  renameKey: (db: number, old_key: string, new_key: string): Promise<void> =>
    request.put(`/redis/data/key/rename?db=${db}`, { old_key, new_key }),
};

export const cliApi = {
  exec: (command: string, db = 0, force = false): Promise<any> =>
    request.post('/redis/cli/exec', { command, db, force }),
};

export const clientApi = {
  getClients: (): Promise<ConnectedClient[]> => request.get('/redis/clients'),
  killClient: (addr: string): Promise<void> => request.post('/redis/clients/kill', { addr }),
};

export const slowlogApi = {
  getLogs: (limit = 100): Promise<SlowLogItem[]> => request.get(`/redis/slowlog?limit=${limit}`),
  reset: (): Promise<void> => request.post('/redis/slowlog/reset'),
};

export const tlsApi = {
  getStatus: (): Promise<TLSStatusResponse> => request.get('/redis/tls/status'),
  enable: (data: { domain: string; email?: string; mode?: string; provider?: string; use_staging?: boolean; force_renew?: boolean }): Promise<any> =>
    request.post('/redis/tls/enable', data),
  disable: (): Promise<void> => request.post('/redis/tls/disable'),
  renew: (): Promise<void> => request.post('/redis/tls/renew'),
  test: (domain?: string): Promise<any> => request.get(`/redis/tls/test?domain=${domain || ''}`),
  deleteCert: (domain: string): Promise<void> => request.delete(`/redis/tls/cert?domain=${encodeURIComponent(domain)}`),
  fixBind: (): Promise<any> => request.post('/redis/tls/fix-bind'),
  switchMode: (mode: 'only_tls' | 'dual'): Promise<any> => request.post('/redis/tls/switch-mode', { mode }),
};

export const backupApi = {
  listBackups: (): Promise<BackupItem[]> => request.get('/redis/backups'),
  createBackup: (): Promise<BackupItem> => request.post('/redis/backups/create'),
  restoreBackup: (backup_id: number): Promise<void> => request.post('/redis/backups/restore', { backup_id }),
  downloadBackupUrl: (id: number): string => `/api/v1/redis/backups/download/${id}`,
  deleteBackup: (id: number): Promise<void> => request.delete(`/redis/backups/${id}`),
};

export const auditApi = {
  getLogs: (page = 1, pageSize = 20, action?: string): Promise<{ total: number; page: number; page_size: number; list: AuditLogItem[] }> =>
    request.get(`/audit/logs?page=${page}&page_size=${pageSize}&action=${action || ''}`),
};

export const aclApi = {
  getUsers: (): Promise<{ users: any[]; requirepass: boolean }> => request.get('/redis/acl/users'),
  setUser: (data: { username: string; password?: string; enabled: boolean; rules?: string }): Promise<void> =>
    request.post('/redis/acl/user', data),
  deleteUser: (username: string): Promise<void> => request.delete(`/redis/acl/user?username=${encodeURIComponent(username)}`),
  setRequirePass: (password: string): Promise<void> => request.post('/redis/acl/requirepass', { password }),
  getRandomPassword: (length = 32): Promise<{ password: string }> => request.get(`/redis/acl/random-password?length=${length}`),
};

export const analyticsApi = {
  getVPNGeo: (db = 0, pattern = 'soga_conn_*'): Promise<VPNGeoSummary> =>
    request.get(`/redis/analytics/vpn-geo?db=${db}&pattern=${encodeURIComponent(pattern)}`),
};

