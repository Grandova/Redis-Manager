import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Space, Tag, Popconfirm, message } from 'antd';
import { FieldTimeOutlined, ReloadOutlined, DeleteOutlined } from '@ant-design/icons';
import { slowlogApi } from '../../api';
import { SlowLogItem } from '../../types';

export const SlowLog: React.FC = () => {
  const [logs, setLogs] = useState<SlowLogItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    fetchSlowLogs();
  }, []);

  const fetchSlowLogs = async () => {
    setLoading(true);
    try {
      const data = await slowlogApi.getLogs(200);
      setLogs(data);
    } finally {
      setLoading(false);
    }
  };

  const handleReset = async () => {
    try {
      await slowlogApi.reset();
      message.success('已清空 Slow Log');
      fetchSlowLogs();
    } catch {
      // Handled
    }
  };

  const columns = [
    {
      title: '序号 (ID)',
      dataIndex: 'id',
      key: 'id',
      width: 90,
    },
    {
      title: '发生时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: '耗时 (Execution Time)',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 140,
      render: (ms: number) => {
        const color = ms > 50 ? 'red' : ms > 10 ? 'volcano' : 'gold';
        return <Tag color={color}>{ms.toFixed(2)} ms</Tag>;
      },
    },
    {
      title: '慢执行命令 (Command)',
      dataIndex: 'command',
      key: 'command',
      render: (cmd: string) => (
        <code style={{ background: 'rgba(0,0,0,0.04)', padding: '2px 6px', borderRadius: '4px' }}>
          {cmd}
        </code>
      ),
    },
    {
      title: '客户端来源 (Client)',
      dataIndex: 'client',
      key: 'client',
      width: 180,
      render: (c: string) => c || '内部/未知',
    },
  ];

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <FieldTimeOutlined style={{ color: '#ef4444' }} />
            <span>Redis 慢查询日志 (Slow Log)</span>
          </div>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchSlowLogs}>
              刷新
            </Button>
            <Popconfirm
              title="确定清空所有 Slow Log 记录？"
              onConfirm={handleReset}
              okText="清空"
              cancelText="取消"
            >
              <Button danger icon={<DeleteOutlined />}>
                清空慢日志
              </Button>
            </Popconfirm>
          </Space>
        }
        style={{ borderRadius: '12px' }}
      >
        <Table
          columns={columns}
          dataSource={logs}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 15 }}
        />
      </Card>
    </div>
  );
};
