import React, { useState, useEffect } from 'react';
import {
  Card,
  Button,
  Space,
  Modal,
  Select,
  Typography,
  Descriptions,
  Switch,
  Input,
  message,
  Alert,
  Tag,
  Divider,
} from 'antd';
import {
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  SyncOutlined,
  DownloadOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { redisApi } from '../../api';
import { RedisDiscovery } from '../../types';
import { StatusBadge } from '../../components/StatusBadge';

const { Title, Text, Paragraph } = Typography;

export const Service: React.FC = () => {
  const [discovery, setDiscovery] = useState<RedisDiscovery | null>(null);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [logs, setLogs] = useState<string[]>([]);

  // Install modal
  const [installModalVisible, setInstallModalVisible] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState('7.x');
  const [installing, setInstalling] = useState(false);

  // Uninstall modal
  const [uninstallModalVisible, setUninstallModalVisible] = useState(false);
  const [uninstallCode, setUninstallCode] = useState('');

  useEffect(() => {
    fetchDiscovery();
    fetchLogs();
  }, []);

  const fetchDiscovery = async () => {
    setLoading(true);
    try {
      const data = await redisApi.getDiscovery();
      setDiscovery(data);
    } finally {
      setLoading(false);
    }
  };

  const fetchLogs = async () => {
    try {
      const res = await redisApi.getLogs(100);
      setLogs(res.lines || []);
    } catch {
      // Ignored
    }
  };

  const handleServiceAction = async (action: 'start' | 'stop' | 'restart' | 'reload') => {
    setActionLoading(action);
    try {
      if (action === 'start') await redisApi.start(discovery?.service_name);
      if (action === 'stop') await redisApi.stop(discovery?.service_name);
      if (action === 'restart') await redisApi.restart(discovery?.service_name);
      if (action === 'reload') await redisApi.reload(discovery?.service_name);

      message.success(`操作 ${action.toUpperCase()} 执行成功`);
      await fetchDiscovery();
      fetchLogs();
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleAutoStart = async (checked: boolean) => {
    setActionLoading('autostart');
    try {
      await redisApi.setAutoStart(checked, discovery?.service_name);
      message.success(checked ? '已开启开机自启' : '已关闭开机自启');
      fetchDiscovery();
    } finally {
      setActionLoading(null);
    }
  };

  const handleInstall = async () => {
    setInstalling(true);
    try {
      await redisApi.install(selectedVersion);
      message.success(`Redis ${selectedVersion} 安装完成并已成功启动！`);
      setInstallModalVisible(false);
      fetchDiscovery();
      fetchLogs();
    } finally {
      setInstalling(false);
    }
  };

  const handleUninstall = async () => {
    if (uninstallCode !== 'UNINSTALL_REDIS') {
      message.error('验证码输入不正确');
      return;
    }
    setActionLoading('uninstall');
    try {
      await redisApi.uninstall(uninstallCode);
      message.success('Redis 已彻底从宿主机卸载');
      setUninstallModalVisible(false);
      setUninstallCode('');
      fetchDiscovery();
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div>
      {/* Uninstalled Notice & Wizard */}
      {!discovery?.is_installed && (
        <Alert
          message="检测到当前服务器尚未安装 Redis"
          description="系统支持一键安装宿主机原生 Redis 服务，自动配置 systemd 服务化并设置开机自启。"
          type="warning"
          showIcon
          action={
            <Button
              type="primary"
              danger
              icon={<DownloadOutlined />}
              onClick={() => setInstallModalVisible(true)}
            >
              一键安装 Redis
            </Button>
          }
          style={{ marginBottom: '20px', borderRadius: '12px', padding: '16px 20px' }}
        />
      )}

      {/* Service Control Card */}
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <span>Redis 服务管理</span>
            {discovery && (
              <StatusBadge
                status={discovery.is_running ? 'success' : 'error'}
                text={discovery.is_running ? '运行中' : '已停止'}
              />
            )}
          </div>
        }
        extra={
          <Space>
            <Button
              icon={<SyncOutlined spin={loading} />}
              onClick={() => {
                fetchDiscovery();
                fetchLogs();
              }}
            >
              刷新状态
            </Button>
          </Space>
        }
        style={{ borderRadius: '12px', marginBottom: '20px' }}
      >
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', marginBottom: '24px' }}>
          <Button
            type="primary"
            icon={<PlayCircleOutlined />}
            loading={actionLoading === 'start'}
            disabled={!discovery?.is_installed || discovery?.is_running}
            onClick={() => handleServiceAction('start')}
            style={{ backgroundColor: '#16a34a', borderColor: '#16a34a' }}
          >
            启动服务
          </Button>
          <Button
            danger
            icon={<PauseCircleOutlined />}
            loading={actionLoading === 'stop'}
            disabled={!discovery?.is_installed || !discovery?.is_running}
            onClick={() => {
              Modal.confirm({
                title: '确认停止 Redis 服务？',
                content: '停止服务后，所有连接到 Redis 的业务系统将立即中断！',
                okText: '确认停止',
                okType: 'danger',
                cancelText: '取消',
                onOk: () => handleServiceAction('stop'),
              });
            }}
          >
            停止服务
          </Button>
          <Button
            icon={<ReloadOutlined />}
            loading={actionLoading === 'restart'}
            disabled={!discovery?.is_installed}
            onClick={() => handleServiceAction('restart')}
          >
            重启服务
          </Button>
          <Button
            icon={<SyncOutlined />}
            loading={actionLoading === 'reload'}
            disabled={!discovery?.is_installed || !discovery?.is_running}
            onClick={() => handleServiceAction('reload')}
          >
            平滑重载
          </Button>
          <Button
            danger
            type="text"
            icon={<DeleteOutlined />}
            disabled={!discovery?.is_installed}
            onClick={() => setUninstallModalVisible(true)}
            style={{ marginLeft: 'auto' }}
          >
            卸载 Redis
          </Button>
        </div>

        <Divider style={{ margin: '16px 0' }} />

        {/* Detailed Host Information */}
        <Descriptions column={{ xs: 1, sm: 2, md: 3 }} bordered size="small">
          <Descriptions.Item label="服务名称 (systemd)">
            <code>{discovery?.service_name || '-'}</code>
          </Descriptions.Item>
          <Descriptions.Item label="Redis 版本">
            <Tag color="blue">{discovery?.version || '-'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="进程 PID">{discovery?.pid || '-'}</Descriptions.Item>
          <Descriptions.Item label="服务监听端口">
            {discovery?.port || 6379} {discovery?.tls_port ? ` (TLS: ${discovery.tls_port})` : ''}
          </Descriptions.Item>
          <Descriptions.Item label="开机自启动">
            <Switch
              checked={discovery?.auto_start}
              loading={actionLoading === 'autostart'}
              onChange={handleToggleAutoStart}
              checkedChildren="已开启"
              unCheckedChildren="已关闭"
            />
          </Descriptions.Item>
          <Descriptions.Item label="运行时间">{discovery?.uptime || '-'}</Descriptions.Item>
          <Descriptions.Item label="配置文件路径">
            <Text copyable style={{ fontSize: '12px' }}>
              {discovery?.config_file || '-'}
            </Text>
          </Descriptions.Item>
          <Descriptions.Item label="Server 程序路径">
            <Text copyable style={{ fontSize: '12px' }}>
              {discovery?.server_bin || '-'}
            </Text>
          </Descriptions.Item>
          <Descriptions.Item label="数据存储目录">
            <Text copyable style={{ fontSize: '12px' }}>
              {discovery?.data_dir || '-'}
            </Text>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* Systemd / Service Logs */}
      <Card
        title="Redis 运行日志 (最近 100 行)"
        extra={
          <Button size="small" icon={<SyncOutlined />} onClick={fetchLogs}>
            刷新日志
          </Button>
        }
        style={{ borderRadius: '12px' }}
      >
        <div
          style={{
            backgroundColor: '#0f172a',
            color: '#e2e8f0',
            padding: '16px',
            borderRadius: '8px',
            fontFamily: 'monospace',
            fontSize: '12px',
            height: '350px',
            overflowY: 'auto',
            whiteSpace: 'pre-wrap',
            lineHeight: 1.6,
          }}
        >
          {logs.length > 0 ? (
            logs.map((line, idx) => (
              <div key={idx} style={{ color: line.includes('error') || line.includes('Fail') ? '#f87171' : line.includes('warn') ? '#fbbf24' : '#cbd5e1' }}>
                {line}
              </div>
            ))
          ) : (
            <div style={{ color: '#64748b' }}>暂无日志记录</div>
          )}
        </div>
      </Card>

      {/* Install Modal */}
      <Modal
        title="安装原生 Redis 服务"
        open={installModalVisible}
        onOk={handleInstall}
        confirmLoading={installing}
        onCancel={() => setInstallModalVisible(false)}
        okText={installing ? '正在安装...' : '开始安装'}
        cancelText="取消"
      >
        <Paragraph>请选择要安装的 Redis 版本。系统将通过官方源自动构建、创建 systemd 服务并启动：</Paragraph>
        <Select
          style={{ width: '100%', marginBottom: '16px' }}
          value={selectedVersion}
          onChange={setSelectedVersion}
          options={[
            { label: 'Redis 7.x (官方推荐稳定版)', value: '7.x' },
            { label: 'Redis 8.x (最新特性版本)', value: '8.x' },
            { label: '系统源最新版 (APT Default)', value: 'latest' },
          ]}
        />
        <Alert
          type="info"
          message="安装过程会自动创建 /var/lib/redis 数据目录与 /etc/redis/redis.conf 标准配置。"
          showIcon
        />
      </Modal>

      {/* Uninstall Modal */}
      <Modal
        title="卸载 Redis 确认"
        open={uninstallModalVisible}
        onOk={handleUninstall}
        confirmLoading={actionLoading === 'uninstall'}
        onCancel={() => {
          setUninstallModalVisible(false);
          setUninstallCode('');
        }}
        okText="彻底卸载"
        okType="danger"
        cancelText="取消"
      >
        <Alert
          type="error"
          showIcon
          message="警告：卸载将停止 Redis 进程并清除相关包。"
          description="为防止误操作，请输入验证码 UNINSTALL_REDIS 确认卸载："
          style={{ marginBottom: '16px' }}
        />
        <Input
          placeholder="请输入 UNINSTALL_REDIS"
          value={uninstallCode}
          onChange={(e) => setUninstallCode(e.target.value)}
        />
      </Modal>
    </div>
  );
};
