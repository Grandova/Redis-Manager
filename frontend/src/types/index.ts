export interface AdminUser {
  id: number;
  username: string;
  nickname: string;
  email?: string;
  last_login_at?: string;
  last_login_ip?: string;
}

export interface SystemInfo {
  os: string;
  platform: string;
  kernel: string;
  arch: string;
  hostname: string;
  cpu_count: number;
  cpu_model: string;
  cpu_usage: number;
  total_memory: number;
  used_memory: number;
  free_memory: number;
  mem_percent: number;
  uptime: number;
  public_ipv4: string;
  local_ips: string[];
}

export interface PortCheckResult {
  port: number;
  in_use: boolean;
  process_name: string;
  pid: number;
}

export interface DomainDNSResult {
  domain: string;
  ipv4_list: string[];
  ipv6_list: string[];
  server_ip: string;
  is_matched: boolean;
  match_error?: string;
}

export interface RedisDiscovery {
  is_installed: boolean;
  is_running: boolean;
  server_bin: string;
  cli_bin: string;
  config_file: string;
  service_name: string;
  version: string;
  port: number;
  tls_port: number;
  pid: number;
  uptime: string;
  data_dir: string;
  log_file: string;
  auto_start: boolean;
  mode: string;
}

export interface LiveMetrics {
  timestamp: number;
  used_memory: number;
  used_memory_rss: number;
  used_memory_peak: number;
  mem_frag_ratio: number;
  connected_clients: number;
  ops_per_sec: number;
  total_keys: number;
  hit_rate: number;
  keyspace_hits: number;
  keyspace_misses: number;
  net_input_kbps: number;
  net_output_kbps: number;
  total_net_input_bytes: number;
  total_net_output_bytes: number;
  cpu_sys: number;
  cpu_user: number;
  rdb_last_bgsave_status: string;
  rdb_last_save_time: number;
  aof_enabled: boolean;
  uptime_in_seconds: number;
}

export interface HistoryMetrics {
  timestamps: string[];
  ops: number[];
  memory_mb: number[];
  net_in_kbps: number[];
  net_out_kbps: number[];
  hit_rate: number[];
}

export interface FieldPreview {
  field: string;
  value: string;
  ttl?: number;
}

export interface KeyItem {
  key: string;
  type: string;
  ttl: number;
  field_ttl?: number;
  size?: number;
  length?: number;
  value_preview?: string;
  fields?: FieldPreview[];
}

export interface KeyDetail {
  key: string;
  type: string;
  ttl: number;
  field_ttl?: number;
  field_ttls?: Record<string, number>;
  encoding?: string;
  memory?: number;
  value: any;
  length?: number;
}

export interface VPNNodeInfo {
  user_id: string;
  key: string;
  ip: string;
  country: string;
  prov: string;
  province: string;
  city: string;
  isp: string;
  ttl: number;
  raw_value: string;
  updated_at: string;
}

export interface ProvinceStat {
  name: string;
  prov: string;
  region: string;
  count: number;
  percent: number;
  ips: string[];
  users: string[];
  cities: string[];
  isps: string[];
}

export interface RegionStat {
  name: string;
  count: number;
  percent: number;
  provinces: ProvinceStat[];
}

export interface VPNGeoSummary {
  total_users: number;
  total_connections: number;
  total_ips: number;
  total_provinces: number;
  foreign_count: number;
  lan_count: number;
  pattern: string;
  db: number;
  provinces: ProvinceStat[];
  regions: RegionStat[];
  nodes: VPNNodeInfo[];
}

export interface DBStat {
  db: number;
  keys: number;
  avg_ttl: number;
}

export interface ConnectedClient {
  id: string;
  addr: string;
  ip: string;
  port: string;
  name: string;
  age: number;
  idle: number;
  db: number;
  cmd: string;
  flags: string;
}

export interface SlowLogItem {
  id: number;
  timestamp: string;
  duration: number;
  duration_ms: number;
  command: string;
  client: string;
}

export interface CertInfo {
  domain: string;
  issuer: string;
  subject: string;
  not_before: string;
  not_after: string;
  days_remaining: number;
  cert_path: string;
  key_path: string;
  ca_path?: string;
  is_expired: boolean;
}

export interface HandshakeResult {
  success: boolean;
  tls_version?: string;
  cipher_suite?: string;
  server_name?: string;
  error?: string;
  warn_message?: string;
  local_success?: boolean;
}

export interface TLSStatusResponse {
  tls_enabled: boolean;
  mode?: 'only_tls' | 'dual';
  port: number;
  tls_port: number;
  has_cert: boolean;
  cert?: CertInfo;
  handshake?: HandshakeResult;
  auto_renew: boolean;
  bind_warning?: string;
}

export interface BackupItem {
  id: number;
  instance_id: number;
  file_name: string;
  file_path: string;
  file_size: number;
  redis_version: string;
  backup_type: string;
  created_at: string;
}

export interface ConfigBackupItem {
  id: number;
  instance_id: number;
  backup_path: string;
  remark: string;
  content_sha256: string;
  created_at: string;
}

export interface AuditLogItem {
  id: number;
  user_id: number;
  username: string;
  action: string;
  resource: string;
  details: string;
  ip: string;
  user_agent: string;
  status: string;
  created_at: string;
}

export interface ACLUser {
  username: string;
  enabled: boolean;
  flags: string[];
  rules: string;
}
