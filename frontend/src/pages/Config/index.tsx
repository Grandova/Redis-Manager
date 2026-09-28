import React, { useState, useEffect } from 'react';
import {
  Card,
  Tabs,
  Form,
  Input,
  Select,
  Button,
  Table,
  Space,
  Modal,
  message,
  Typography,
  Alert,
  Popconfirm,
  Tag,
} from 'antd';
import {
  SettingOutlined,
  SaveOutlined,
  RollbackOutlined,
  HistoryOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { configApi } from '../../api';
import { ConfigBackupItem } from '../../types';

const { Title, Text, Paragraph } = Typography;

export const Config: React.FC = () => {
  const [activeTab, setActiveTab] = useState('visual');
  const [visualForm] = Form.useForm();
  const [rawContent, setRawContent] = useState('');
  const [configPath, setConfigPath] = useState('/etc/redis/redis.conf');
  const [saving, setSaving] = useState(false);
  const [backups, setBackups] = useState<ConfigBackupItem[]>([]);
  const [backupsLoading, setBackupsLoading] = useState(false);

  useEffect(() => {
    fetchConfig();
    fetchBackups();
  }, []);

  const fetchConfig = async () => {
    try {
      const res = await configApi.getConfig();
      setConfigPath(res.config_path);
      visualForm.setFieldsValue({
        bind: res.items.bind || '127.0.0.1',
        port: res.items.port || '6379',
        timeout: res.items.timeout || '0',
        maxmemory: res.items.maxmemory || '',
        maxmemory_policy: res.items['maxmemory-policy'] || 'noeviction',
        maxclients: res.items.maxclients || '10000',
        appendonly: res.items.appendonly || 'no',
        loglevel: res.items.loglevel || 'notice',
        protected_mode: res.items['protected-mode'] || 'yes',
        dir: res.items.dir || '/var/lib/redis',
        dbfilename: res.items.dbfilename || 'dump.rdb',
      });

      const raw = await configApi.getRawConfig();
      setRawContent(raw.content);
    } catch {
      // Ignored
    }
  };

  const fetchBackups = async () => {
    setBackupsLoading(true);
    try {
      const data = await configApi.getBackups();
      setBackups(data);
    } finally {
      setBackupsLoading(false);
    }
  };

  const handleSaveVisual = async (values: any) => {
    setSaving(true);
    try {
      await configApi.updateConfig(values);
      message.success('配置已保存并平滑生效');
      fetchConfig();
      fetchBackups();
    } catch {
      // Handled
    } finally {
      setSaving(false);
    }
  };

  const handleSaveRaw = async () => {
    setSaving(true);
    try {
      await configApi.updateRawConfig(rawContent, '手动在线编辑 redis.conf');
      message.success('配置已通过校验、生成备份并成功应用生效');
      fetchConfig();
      fetchBackups();
    } catch {
      // Handled
    } finally {
      setSaving(false);
    }
  };

  const handleRollback = async (backupId: number) => {
    try {
      await configApi.rollback(backupId);
      message.success(`已成功回滚至历史版本 #${backupId}`);
      fetchConfig();
      fetchBackups();
    } catch {
      // Handled
    }
  };

  const backupColumns = [
    {
      title: '备份 ID',
      dataIndex: 'id',
      key: 'id',
      width: 90,
    },
    {
      title: '备份时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: '备注说明',
      dataIndex: 'remark',
      key: 'remark',
      render: (r: string) => <Tag color="blue">{r}</Tag>,
    },
    {
      title: '配置指纹 (SHA256)',
      dataIndex: 'content_sha256',
      key: 'content_sha256',
      render: (sha: string) => <code>{sha.slice(0, 16)}...</code>,
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_: any, record: ConfigBackupItem) => (
        <Popconfirm
          title="确认回滚至该历史版本？"
          description="系统将自动重启 Redis 并应用此配置"
          onConfirm={() => handleRollback(record.id)}
          okText="立即回滚"
          cancelText="取消"
        >
          <Button size="small" type="primary" ghost icon={<RollbackOutlined />}>
            一键回滚
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <SettingOutlined style={{ color: '#ef4444' }} />
            <span>Redis 核心配置管理</span>
            <Text type="secondary" style={{ fontSize: '12px' }}>
              ({configPath})
            </Text>
          </div>
        }
        style={{ borderRadius: '12px' }}
      >
        <Alert
          type="info"
          showIcon
          message="配置原子化与故障自动回滚保障"
          description="每次修改均会自动创建历史快照并进行语法验证；若重启后探针探测失败，系统将自动回滚恢复旧配置，杜绝宕机。"
          style={{ marginBottom: '20px', borderRadius: '8px' }}
        />

        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'visual',
              label: '可视化设置',
              children: (
                <Form form={visualForm} layout="vertical" onFinish={handleSaveVisual} style={{ maxWidth: '900px' }}>
                  <Title level={5}>网络设置 (Network)</Title>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '16px' }}>
                    <Form.Item name="bind" label="绑定地址 (bind)">
                      <Input placeholder="127.0.0.1 -::1" />
                    </Form.Item>
                    <Form.Item name="port" label="监听端口 (port)">
                      <Input placeholder="6379" />
                    </Form.Item>
                    <Form.Item name="timeout" label="空闲客户端超时 (timeout / 秒)">
                      <Input placeholder="0 (永不超时)" />
                    </Form.Item>
                    <Form.Item name="protected_mode" label="保护模式 (protected-mode)">
                      <Select
                        options={[
                          { label: '开启 (yes)', value: 'yes' },
                          { label: '关闭 (no)', value: 'no' },
                        ]}
                      />
                    </Form.Item>
                  </div>

                  <Title level={5} style={{ marginTop: '16px' }}>
                    内存与限制 (Memory & Limits)
                  </Title>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '16px' }}>
                    <Form.Item name="maxmemory" label="最大内存限制 (maxmemory)">
                      <Input placeholder="例如: 2gb, 512mb (留空为无限制)" />
                    </Form.Item>
                    <Form.Item name="maxmemory_policy" label="内存淘汰策略 (maxmemory-policy)">
                      <Select
                        options={[
                          { label: 'noeviction (达到上限拒绝写入)', value: 'noeviction' },
                          { label: 'allkeys-lru (所有键最少使用淘汰)', value: 'allkeys-lru' },
                          { label: 'volatile-lru (过期键最少使用淘汰)', value: 'volatile-lru' },
                          { label: 'allkeys-lfu (所有键最少频次淘汰)', value: 'allkeys-lfu' },
                          { label: 'volatile-lfu (过期键最少频次淘汰)', value: 'volatile-lfu' },
                          { label: 'allkeys-random (随机淘汰)', value: 'allkeys-random' },
                          { label: 'volatile-ttl (优先淘汰TTL最短的键)', value: 'volatile-ttl' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item name="maxclients" label="最大并发客户端数 (maxclients)">
                      <Input placeholder="10000" />
                    </Form.Item>
                  </div>

                  <Title level={5} style={{ marginTop: '16px' }}>
                    持久化与日志 (Persistence & Logs)
                  </Title>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '16px' }}>
                    <Form.Item name="appendonly" label="AOF 持久化 (appendonly)">
                      <Select
                        options={[
                          { label: '开启 (yes)', value: 'yes' },
                          { label: '关闭 (no)', value: 'no' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item name="loglevel" label="日志级别 (loglevel)">
                      <Select
                        options={[
                          { label: 'notice (标准生产推荐)', value: 'notice' },
                          { label: 'warning (仅告警)', value: 'warning' },
                          { label: 'verbose (详尽)', value: 'verbose' },
                          { label: 'debug (调试模式)', value: 'debug' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item name="dir" label="工作目录 / RDB 存放路径 (dir)">
                      <Input placeholder="/var/lib/redis" />
                    </Form.Item>
                    <Form.Item name="dbfilename" label="RDB 快照文件名 (dbfilename)">
                      <Input placeholder="dump.rdb" />
                    </Form.Item>
                  </div>

                  <Form.Item style={{ marginTop: '24px' }}>
                    <Button
                      type="primary"
                      htmlType="submit"
                      icon={<SaveOutlined />}
                      loading={saving}
                      size="large"
                      style={{ backgroundColor: '#ef4444', borderColor: '#ef4444', borderRadius: '8px' }}
                    >
                      保存配置并应用
                    </Button>
                  </Form.Item>
                </Form>
              ),
            },
            {
              key: 'raw',
              label: '原始 redis.conf 编辑',
              children: (
                <div>
                  <Paragraph type="secondary">
                    可在此直接修改完整的 redis.conf 文本。点击保存后，系统将自动备份、校验并热重启生效：
                  </Paragraph>
                  <Input.TextArea
                    rows={22}
                    value={rawContent}
                    onChange={(e) => setRawContent(e.target.value)}
                    style={{
                      fontFamily: 'Consolas, Monaco, monospace',
                      fontSize: '13px',
                      backgroundColor: '#0f172a',
                      color: '#f8fafc',
                      borderRadius: '8px',
                      padding: '16px',
                    }}
                  />
                  <div style={{ marginTop: '16px' }}>
                    <Button
                      type="primary"
                      icon={<SaveOutlined />}
                      loading={saving}
                      onClick={handleSaveRaw}
                      size="large"
                      style={{ backgroundColor: '#ef4444', borderColor: '#ef4444', borderRadius: '8px' }}
                    >
                      安全保存并应用
                    </Button>
                  </div>
                </div>
              ),
            },
            {
              key: 'history',
              label: '历史快照与回滚',
              children: (
                <Table
                  columns={backupColumns}
                  dataSource={backups}
                  rowKey="id"
                  loading={backupsLoading}
                  pagination={{ pageSize: 10 }}
                />
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
};
