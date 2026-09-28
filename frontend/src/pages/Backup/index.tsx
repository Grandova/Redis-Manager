import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Popconfirm,
  Modal,
  message,
  Typography,
  Alert,
} from 'antd';
import {
  CloudUploadOutlined,
  DownloadOutlined,
  RollbackOutlined,
  DeleteOutlined,
  ReloadOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { backupApi } from '../../api';
import { BackupItem } from '../../types';

const { Text, Paragraph } = Typography;

export const Backup: React.FC = () => {
  const [backups, setBackups] = useState<BackupItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [restoringId, setRestoringId] = useState<number | null>(null);

  useEffect(() => {
    fetchBackups();
  }, []);

  const fetchBackups = async () => {
    setLoading(true);
    try {
      const data = await backupApi.listBackups();
      setBackups(data);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    setCreating(true);
    try {
      await backupApi.createBackup();
      message.success('RDB 备份创建成功');
      fetchBackups();
    } catch {
      // Handled
    } finally {
      setCreating(false);
    }
  };

  const handleRestore = async (id: number) => {
    Modal.confirm({
      title: '确认安全恢复此备份？',
      content: (
        <div>
          <Paragraph type="danger">
            恢复操作将覆盖当前活跃的 Redis 数据。
          </Paragraph>
          <Paragraph>
            【安全保障机制】系统在执行恢复前，会<b>自动强制生成一份当前实时数据的紧急快照</b>，确保任何情况下数据不丢失。
          </Paragraph>
        </div>
      ),
      okText: '确认恢复',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        setRestoringId(id);
        try {
          await backupApi.restoreBackup(id);
          message.success('数据已成功恢复，Redis 服务已重启');
          fetchBackups();
        } finally {
          setRestoringId(null);
        }
      },
    });
  };

  const handleDelete = async (id: number) => {
    try {
      await backupApi.deleteBackup(id);
      message.success('备份已删除');
      fetchBackups();
    } catch {
      // Handled
    }
  };

  const formatSize = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const columns = [
    {
      title: '序号',
      dataIndex: 'id',
      key: 'id',
      width: 70,
    },
    {
      title: '备份文件名',
      dataIndex: 'file_name',
      key: 'file_name',
      render: (name: string) => <code style={{ fontWeight: 600 }}>{name}</code>,
    },
    {
      title: '文件大小',
      dataIndex: 'file_size',
      key: 'file_size',
      width: 120,
      render: (size: number) => <Tag color="blue">{formatSize(size)}</Tag>,
    },
    {
      title: 'Redis 版本',
      dataIndex: 'redis_version',
      key: 'redis_version',
      width: 130,
      render: (v: string) => <Tag>{v || 'Redis'}</Tag>,
    },
    {
      title: '备份时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      render: (_: any, record: BackupItem) => (
        <Space size="small">
          <Button
            size="small"
            type="link"
            icon={<DownloadOutlined />}
            href={backupApi.downloadBackupUrl(record.id)}
            target="_blank"
          >
            下载
          </Button>
          <Button
            size="small"
            type="link"
            icon={<RollbackOutlined />}
            loading={restoringId === record.id}
            onClick={() => handleRestore(record.id)}
          >
            恢复
          </Button>
          <Popconfirm
            title="确认删除该备份文件？"
            onConfirm={() => handleDelete(record.id)}
            okText="删除"
            cancelText="取消"
          >
            <Button size="small" type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <CloudUploadOutlined style={{ color: '#ef4444' }} />
            <span>Redis 数据备份与快照管理</span>
          </div>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchBackups}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              loading={creating}
              onClick={handleCreate}
              style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}
            >
              创建 RDB 备份
            </Button>
          </Space>
        }
        style={{ borderRadius: '12px' }}
      >
        <Alert
          type="info"
          showIcon
          message="灾难恢复与防数据丢失机制"
          description="点击创建备份将触发 Redis BGSAVE 异步持久化并自动归档 dump.rdb 文件。恢复备份前，系统强制前置自动备份当前数据，绝无数据覆水难收的风险。"
          style={{ marginBottom: '16px', borderRadius: '8px' }}
        />

        <Table
          columns={columns}
          dataSource={backups}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>
    </div>
  );
};
