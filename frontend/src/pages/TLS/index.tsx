import React, { useState, useEffect } from 'react';
import {
  Card,
  Input,
  Button,
  Radio,
  Space,
  Typography,
  Alert,
  Descriptions,
  Tag,
  Modal,
  message,
  Divider,
} from 'antd';
import {
  SafetyCertificateOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  ThunderboltOutlined,
  StopOutlined,
  DeleteOutlined,
  GlobalOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { tlsApi, systemApi } from '../../api';
import { TLSStatusResponse, DomainDNSResult, PortCheckResult } from '../../types';
import { StatusBadge } from '../../components/StatusBadge';

const { Title, Text, Paragraph } = Typography;

export const TLS: React.FC = () => {
  const [tlsStatus, setTlsStatus] = useState<TLSStatusResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [domainInput, setDomainInput] = useState('');
  const [mode, setMode] = useState<'only_tls' | 'dual'>('only_tls');
  const [applying, setApplying] = useState(false);

  // Pre-flight check states
  const [dnsResult, setDnsResult] = useState<DomainDNSResult | null>(null);
  const [dnsChecking, setDnsChecking] = useState(false);
  const [port80Result, setPort80Result] = useState<PortCheckResult | null>(null);

  // Testing probe
  const [testingProbe, setTestingProbe] = useState(false);

  useEffect(() => {
    fetchStatus();
    checkPort80();
  }, []);

  const fetchStatus = async () => {
    setLoading(true);
    try {
      const data = await tlsApi.getStatus();
      setTlsStatus(data);
      if (data.cert?.domain) {
        setDomainInput(data.cert.domain);
      }
    } finally {
      setLoading(false);
    }
  };

  const checkPort80 = async () => {
    try {
      const res = await systemApi.checkPort(80);
      setPort80Result(res);
    } catch {
      // Ignored
    }
  };

  const handleCheckDNS = async () => {
    if (!domainInput.trim()) {
      message.error('请先输入需要配置的域名');
      return;
    }
    setDnsChecking(true);
    try {
      const res = await systemApi.checkDNS(domainInput.trim());
      setDnsResult(res);
      if (res.is_matched) {
        message.success('DNS 解析正确，已匹配当前服务器公网 IP！');
      } else {
        message.warning(res.match_error || '域名解析尚未指向当前服务器');
      }
    } finally {
      setDnsChecking(false);
    }
  };

  const handleEnableTLS = async () => {
    if (!domainInput.trim()) {
      message.error('请输入域名');
      return;
    }

    setApplying(true);
    try {
      const res = await tlsApi.enable({
        domain: domainInput.trim(),
        mode,
        provider: 'letsencrypt',
      });
      message.success(res?.message || 'Redis TLS 证书申请与配置成功！');
      fetchStatus();
    } catch (err: any) {
      // Error message handled in request interceptor
    } finally {
      setApplying(false);
    }
  };

  const handleDisableTLS = async () => {
    Modal.confirm({
      title: '确认关闭 Redis TLS？',
      content: '关闭后，Redis 将恢复为普通明文 TCP 端口 6379 监听，证书文件将被保留。',
      okText: '确认关闭',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setApplying(true);
        try {
          await tlsApi.disable();
          message.success('Redis TLS 已关闭，恢复普通 TCP 监听');
          fetchStatus();
        } finally {
          setApplying(false);
        }
      },
    });
  };

  const handleRenewNow = async () => {
    setApplying(true);
    try {
      await tlsApi.renew();
      message.success('证书续签成功并已重新加载！');
      fetchStatus();
    } finally {
      setApplying(false);
    }
  };

  const handleFixBind = async () => {
    setApplying(true);
    try {
      const res = await tlsApi.fixBind();
      message.success(res?.message || 'Redis 监听地址已成功更新为 0.0.0.0 并重启！');
      fetchStatus();
    } catch {
      // handled
    } finally {
      setApplying(false);
    }
  };

  const handleSwitchMode = async () => {
    const targetMode = tlsStatus?.port === 0 ? 'dual' : 'only_tls';
    const targetText = targetMode === 'dual' ? '双模式兼容 (明文 6379 + TLS 6380)' : '仅 TLS 模式 (仅 6379 TLS，关闭明文)';
    Modal.confirm({
      title: `确认切换为 ${targetText}？`,
      content: '系统将修改 redis.conf 端口配置并自动重启生效，本地已签发的证书文件不受影响。',
      okText: '确认切换',
      onOk: async () => {
        setApplying(true);
        try {
          const res = await tlsApi.switchMode(targetMode);
          message.success(res?.message || '模式切换成功！');
          fetchStatus();
        } finally {
          setApplying(false);
        }
      },
    });
  };

  const handleTestProbe = async () => {
    setTestingProbe(true);
    try {
      const res = await tlsApi.test(domainInput);
      if (res.success) {
        if (res.warn_message) {
          message.warning(res.warn_message, 8);
        } else {
          message.success(`TLS 握手测试正常！协议: ${res.tls_version}, 加密套件: ${res.cipher_suite}`);
        }
      } else {
        message.error(`TLS 握手测试失败: ${res.error}`);
      }
      fetchStatus();
    } finally {
      setTestingProbe(false);
    }
  };

  const handleDeleteCert = async () => {
    if (!tlsStatus?.cert?.domain) return;
    Modal.confirm({
      title: '确认删除证书？',
      content: `将彻底删除域名 ${tlsStatus.cert.domain} 的本地证书文件。`,
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        await tlsApi.deleteCert(tlsStatus.cert!.domain);
        message.success('证书已删除');
        fetchStatus();
      },
    });
  };

  return (
    <div>
      {/* TLS Overview Card */}
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <SafetyCertificateOutlined style={{ color: '#ef4444' }} />
            <span>Redis TLS 自动化安全与证书管理</span>
            {tlsStatus && (
              <StatusBadge
                status={tlsStatus.tls_enabled ? 'success' : 'default'}
                text={tlsStatus.tls_enabled ? 'TLS 已启用' : 'TLS 未启用'}
              />
            )}
          </div>
        }
        extra={
          <Button icon={<SyncOutlined spin={loading} />} onClick={fetchStatus}>
            刷新状态
          </Button>
        }
        style={{ borderRadius: '12px', marginBottom: '20px' }}
      >
        {tlsStatus?.tls_enabled ? (
          <div>
            <Alert
              type="success"
              showIcon
              message="Redis TLS 正在安全保护中"
              description={`所有客户端连接均已通过 TLS/SSL 进行强加密传输。系统后台将每 12 小时自动巡检并在到期前 30 天自动完成 Let's Encrypt 证书续期。`}
              style={{ marginBottom: '16px', borderRadius: '8px' }}
            />

            {tlsStatus.bind_warning && (
              <Alert
                type="warning"
                showIcon
                message="Redis 监听地址需要更新"
                description={
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '10px' }}>
                    <span>{tlsStatus.bind_warning}</span>
                    <Button
                      type="primary"
                      size="small"
                      loading={applying}
                      onClick={handleFixBind}
                      style={{ background: '#f59e0b', borderColor: '#f59e0b', fontWeight: 600 }}
                    >
                      一键切换为公网监听 (bind 0.0.0.0)
                    </Button>
                  </div>
                }
                style={{ marginBottom: '16px', borderRadius: '8px' }}
              />
            )}

            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} bordered size="middle">
              <Descriptions.Item label="绑定域名">{tlsStatus.cert?.domain || '-'}</Descriptions.Item>
              <Descriptions.Item label="证书颁发机构 (CA)">
                <Tag color="green">{tlsStatus.cert?.issuer || "Let's Encrypt"}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="证书到期时间">
                {tlsStatus.cert?.not_after ? new Date(tlsStatus.cert.not_after).toLocaleDateString() : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="剩余有效期">
                <Tag color={Number(tlsStatus.cert?.days_remaining) < 30 ? 'warning' : 'blue'}>
                  {tlsStatus.cert?.days_remaining} 天
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="自动续签">
                <Tag color="cyan">✓ 每天两次巡检</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="当前模式">
                {tlsStatus.port === 0 ? (
                  <Tag color="green">仅 TLS 模式 (仅监听 6379 TLS，明文已关闭)</Tag>
                ) : (
                  <Tag color="blue">双模式兼容 (明文 6379 + TLS 6380)</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="TLS 端口">
                {tlsStatus.tls_port} {tlsStatus.port > 0 ? `(同时开启明文 ${tlsStatus.port})` : '(仅允许 TLS)'}
              </Descriptions.Item>
              <Descriptions.Item label="TLS 协议版本">
                <Tag color="purple">{tlsStatus.handshake?.tls_version || 'TLS 1.3'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="加密套件 (Cipher)">
                <code>{tlsStatus.handshake?.cipher_suite || 'TLS_AES_128_GCM_SHA256'}</code>
              </Descriptions.Item>
              <Descriptions.Item label="握手连通性">
                {tlsStatus.handshake?.success ? (
                  <div>
                    <span style={{ color: '#16a34a', fontWeight: 600 }}>✓ 握手测试通过</span>
                    {tlsStatus.handshake?.warn_message && (
                      <div style={{ fontSize: '12px', color: '#d97706', marginTop: '4px', lineHeight: 1.4 }}>
                        ⚠ {tlsStatus.handshake.warn_message}
                      </div>
                    )}
                  </div>
                ) : (
                  <span style={{ color: '#dc2626' }}>{tlsStatus.handshake?.error || '未握手'}</span>
                )}
              </Descriptions.Item>
            </Descriptions>

            <div style={{ marginTop: '20px', display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
              <Button icon={<ThunderboltOutlined />} loading={testingProbe} onClick={handleTestProbe}>
                重新测试 TLS 握手
              </Button>
              <Button icon={<SwapOutlined />} loading={applying} onClick={handleSwitchMode}>
                {tlsStatus.port === 0 ? '切换为双模式兼容 (6379明文/6380TLS)' : '切换为仅 TLS 模式 (仅6379TLS)'}
              </Button>
              <Button icon={<SyncOutlined />} loading={applying} onClick={handleRenewNow}>
                立即手动续签
              </Button>
              <Button danger icon={<StopOutlined />} loading={applying} onClick={handleDisableTLS}>
                关闭 TLS (恢复普通连接)
              </Button>
              <Button type="text" danger icon={<DeleteOutlined />} onClick={handleDeleteCert} style={{ marginLeft: 'auto' }}>
                删除证书文件
              </Button>
            </div>
          </div>
        ) : (
          <div>
            <Paragraph>
              用户只需将域名 A / AAAA 记录解析到当前服务器公网 IP，在下方输入域名后点击<b>开启 TLS</b>
              ，系统将自动通过 ACME 完成证书申请、配置 redis.conf、重启并测试握手，全程无须手动干预！
            </Paragraph>

            {/* Existing local certificate prompt */}
            {tlsStatus?.has_cert && tlsStatus?.cert && !tlsStatus.cert.is_expired && (
              <Alert
                type="info"
                showIcon
                message="检测到本地已存在有效证书"
                description={
                  <div>
                    域名 <code>{tlsStatus.cert.domain}</code> 已在本地拥有可用证书（颁发机构: {tlsStatus.cert.issuer || "Let's Encrypt"}，剩余有效期: <b>{tlsStatus.cert.days_remaining}</b> 天）。
                    <br />
                    开启时系统将<b>直接复用本地证书秒级生效</b>，无须重新向 CA 申请，不受 80 端口占用或证书申请频率限制。
                  </div>
                }
                style={{ marginBottom: '16px', borderRadius: '8px' }}
              />
            )}

            {/* Port 80 Check Alert */}
            {port80Result?.in_use && (!tlsStatus?.has_cert || tlsStatus?.cert?.is_expired) && (
              <Alert
                type="warning"
                showIcon
                message="80 端口占用提示"
                description={`检测到服务器 80 端口正在被 ${port80Result.process_name || '其他服务'} (PID: ${port80Result.pid}) 占用。ACME HTTP-01 验证需要使用 80 端口。若申请受阻，请先临时停止该服务。`}
                style={{ marginBottom: '16px', borderRadius: '8px' }}
              />
            )}

            <div style={{ maxWidth: '680px', marginTop: '20px' }}>
              <div style={{ marginBottom: '16px' }}>
                <Text strong>1. 输入解析到当前服务器的域名：</Text>
                <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
                  <Input
                    prefix={<GlobalOutlined style={{ color: '#888' }} />}
                    placeholder="例如: redis.example.com"
                    value={domainInput}
                    onChange={(e) => setDomainInput(e.target.value)}
                    size="large"
                    style={{ borderRadius: '8px' }}
                  />
                  <Button
                    icon={<SyncOutlined spin={dnsChecking} />}
                    onClick={handleCheckDNS}
                    size="large"
                    style={{ borderRadius: '8px' }}
                  >
                    预检 DNS
                  </Button>
                </div>
              </div>

              {/* DNS Check Result Card */}
              {dnsResult && (
                <div
                  style={{
                    backgroundColor: dnsResult.is_matched ? 'rgba(34, 197, 94, 0.08)' : 'rgba(239, 68, 68, 0.08)',
                    border: `1px solid ${dnsResult.is_matched ? '#bbf7d0' : '#fecaca'}`,
                    padding: '12px 16px',
                    borderRadius: '8px',
                    marginBottom: '16px',
                  }}
                >
                  <div style={{ fontWeight: 600, color: dnsResult.is_matched ? '#15803d' : '#b91c1c' }}>
                    {dnsResult.is_matched ? '✓ DNS 解析正确匹配' : '✕ DNS 解析未匹配'}
                  </div>
                  <div style={{ fontSize: '13px', marginTop: '4px', color: '#4b5563' }}>
                    域名: <code>{dnsResult.domain}</code> • 解析 IP: <code>{dnsResult.ipv4_list.join(', ') || '无'}</code> • 服务器公网 IP: <code>{dnsResult.server_ip}</code>
                  </div>
                  {!dnsResult.is_matched && (
                    <div style={{ fontSize: '12px', color: '#ef4444', marginTop: '4px' }}>
                      {dnsResult.match_error}
                    </div>
                  )}
                </div>
              )}

              <div style={{ marginBottom: '20px' }}>
                <Text strong>2. 选择监听模式：</Text>
                <div style={{ marginTop: '8px' }}>
                  <Radio.Group value={mode} onChange={(e) => setMode(e.target.value)}>
                    <Radio value="only_tls">
                      <b>仅 TLS (推荐)</b>：关闭明文 Redis，仅监听 6379 TLS 端口
                    </Radio>
                    <div style={{ height: '8px' }} />
                    <Radio value="dual">
                      <b>双模式兼容</b>：6379 保持明文 Redis，6380 监听 Redis TLS
                    </Radio>
                  </Radio.Group>
                </div>
              </div>

              <Button
                type="primary"
                size="large"
                icon={<SafetyCertificateOutlined />}
                loading={applying}
                onClick={handleEnableTLS}
                style={{
                  backgroundColor: '#ef4444',
                  borderColor: '#ef4444',
                  height: '46px',
                  borderRadius: '8px',
                  fontWeight: 600,
                  padding: '0 32px',
                }}
              >
                一键开启 Redis TLS
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
};
