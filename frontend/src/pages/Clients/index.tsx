import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Space, Tag, Popconfirm, message, Typography } from 'antd';
import { TeamOutlined, ReloadOutlined, DisconnectOutlined } from '@ant-design/icons';
import { clientApi } from '../../api';
import { ConnectedClient } from '../../types';

export const Clients: React.FC = () => {
  const [clients, setClients] = useState<ConnectedClient[]>([]);
  const [loading, setLoading] = useState<boolean>(false);

  useEffect(() => {
    fetchClients();
    const timer = setInterval(fetchClients, 5000);
    return () => clearInterval(timer);
  }, []);

  const fetchClients = async () => {
    setLoading(true);
    try {
      const data = await clientApi.getClients();
      setClients(data);
    } finally {
      setLoading(false);
    }
  };

  const handleKill = async (addr: string) => {
    try {
      await clientApi.killClient(addr);
      message.success(`已断开客户端连接: ${addr}`);
      fetchClients();
    } catch {
      // Handled
    }
  };

  const columns = [
    {
      title: '客户端 ID',
      dataIndex: 'id',
      key: 'id',
      width: 90,
    },
    {
      title: 'IP 地址',
      dataIndex: 'ip',
      key: 'ip',
      render: (ip: string, r: ConnectedClient) => (
        <span>
          <span style={{ fontWeight: 600 }}>{ip}</span>:{r.port}
        </span>
      ),
    },
    {
      title: '连接名称 (Name)',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => name || '-',
    },
    {
      title: '当前 DB',
      dataIndex: 'db',
      key: 'db',
      width: 80,
      render: (db: number) => <Tag color="blue">DB{db}</Tag>,
    },
    {
      title: '连接时长 (Age)',
      dataIndex: 'age',
      key: 'age',
      render: (age: number) => `${age} 秒`,
    },
    {
      title: '空闲时间 (Idle)',
      dataIndex: 'idle',
      key: 'idle',
      render: (idle: number) => `${idle} 秒`,
    },
    {
      title: '最近执行命令',
      dataIndex: 'cmd',
      key: 'cmd',
      render: (cmd: string) => <Tag color="geekblue">{cmd}</Tag>,
    },
    {
      title: 'Flags',
      dataIndex: 'flags',
      key: 'flags',
      width: 90,
      render: (flags: string) => <Tag>{flags}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_: any, record: ConnectedClient) => (
        <Popconfirm
          title="确认断开该客户端连接？"
          description={`客户端: ${record.addr}`}
          onConfirm={() => handleKill(record.addr)}
          okText="断开"
          cancelText="取消"
        >
          <Button size="small" danger icon={<DisconnectOutlined />}>
            断开连接
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
            <TeamOutlined style={{ color: '#ef4444' }} />
            <span>Redis 在线客户端连接 ({clients.length})</span>
          </div>
        }
        extra={
          <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchClients}>
            刷新
          </Button>
        }
        style={{ borderRadius: '12px' }}
      >
        <Table
          columns={columns}
          dataSource={clients}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 15 }}
        />
      </Card>
    </div>
  );
};
