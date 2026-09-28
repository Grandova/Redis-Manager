import React, { useState, useEffect } from 'react';
import { Card, Table, Tag, Button, Typography, Space } from 'antd';
import { HistoryOutlined, ReloadOutlined } from '@ant-design/icons';
import { auditApi } from '../../api';
import { AuditLogItem } from '../../types';

export const Audit: React.FC = () => {
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchLogs(page, pageSize);
  }, [page, pageSize]);

  const fetchLogs = async (p: number, ps: number) => {
    setLoading(true);
    try {
      const res = await auditApi.getLogs(p, ps);
      setLogs(res.list || []);
      setTotal(res.total || 0);
    } finally {
      setLoading(false);
    }
  };

  const getActionTag = (action: string) => {
    if (action.includes('LOGIN')) return <Tag color="blue">登录认证</Tag>;
    if (action.includes('CONFIG')) return <Tag color="purple">配置变更</Tag>;
    if (action.includes('TLS')) return <Tag color="green">TLS 证书</Tag>;
    if (action.includes('BACKUP')) return <Tag color="cyan">备份恢复</Tag>;
    if (action.includes('KEY')) return <Tag color="geekblue">数据管理</Tag>;
    if (action.includes('REDIS')) return <Tag color="volcano">服务控制</Tag>;
    return <Tag>{action}</Tag>;
  };

  const columns = [
    {
      title: '序号',
      dataIndex: 'id',
      key: 'id',
      width: 70,
    },
    {
      title: '操作时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: '管理员',
      dataIndex: 'username',
      key: 'username',
      width: 110,
      render: (u: string) => <span style={{ fontWeight: 600 }}>{u}</span>,
    },
    {
      title: '模块/动作',
      dataIndex: 'action',
      key: 'action',
      width: 140,
      render: (a: string) => getActionTag(a),
    },
    {
      title: '操作详情',
      dataIndex: 'details',
      key: 'details',
      render: (d: string) => <span>{d}</span>,
    },
    {
      title: 'IP 地址',
      dataIndex: 'ip',
      key: 'ip',
      width: 140,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => (
        <Tag color={s === 'SUCCESS' ? 'success' : 'error'}>{s === 'SUCCESS' ? '成功' : '失败'}</Tag>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <HistoryOutlined style={{ color: '#ef4444' }} />
            <span>系统安全与操作审计日志</span>
          </div>
        }
        extra={
          <Button icon={<ReloadOutlined spin={loading} />} onClick={() => fetchLogs(page, pageSize)}>
            刷新
          </Button>
        }
        style={{ borderRadius: '12px' }}
      >
        <Table
          columns={columns}
          dataSource={logs}
          rowKey="id"
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
        />
      </Card>
    </div>
  );
};
