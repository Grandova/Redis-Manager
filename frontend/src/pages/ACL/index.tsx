import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Tag,
  Space,
  Modal,
  Form,
  Input,
  Switch,
  Typography,
  message,
  Popconfirm,
  Alert,
  Divider,
} from 'antd';
import {
  SafetyOutlined,
  UserAddOutlined,
  KeyOutlined,
  ReloadOutlined,
  DeleteOutlined,
  EditOutlined,
  LockOutlined,
} from '@ant-design/icons';
import { aclApi } from '../../api';
import { ACLUser } from '../../types';

const { Title, Text, Paragraph } = Typography;

export const ACL: React.FC = () => {
  const [users, setUsers] = useState<ACLUser[]>([]);
  const [hasRequirePass, setHasRequirePass] = useState(false);
  const [loading, setLoading] = useState(false);

  // RequirePass Modal
  const [passModalVisible, setPassModalVisible] = useState(false);
  const [passInput, setPassInput] = useState('');
  const [passLoading, setPassLoading] = useState(false);

  // Add/Edit ACL User Modal
  const [userModalVisible, setUserModalVisible] = useState(false);
  const [userForm] = Form.useForm();
  const [userLoading, setUserLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    fetchUsers();
  }, []);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const res = await aclApi.getUsers();
      setUsers(res.users || []);
      setHasRequirePass(res.requirepass);
    } finally {
      setLoading(false);
    }
  };

  const handleGenerateRandomPass = async (target: 'requirepass' | 'user') => {
    try {
      const res = await aclApi.getRandomPassword(32);
      if (target === 'requirepass') {
        setPassInput(res.password);
      } else {
        userForm.setFieldValue('password', res.password);
      }
      message.success('已自动生成 32 位强随机安全密码！');
    } catch {
      // Handled
    }
  };

  const handleSaveRequirePass = async () => {
    setPassLoading(true);
    try {
      await aclApi.setRequirePass(passInput.trim());
      message.success('Redis requirepass 密码已更新并写入 redis.conf');
      setPassModalVisible(false);
      fetchUsers();
    } finally {
      setPassLoading(false);
    }
  };

  const handleSaveUser = async (values: any) => {
    setUserLoading(true);
    try {
      await aclApi.setUser(values);
      message.success(`ACL 用户 ${values.username} 保存成功`);
      setUserModalVisible(false);
      userForm.resetFields();
      fetchUsers();
    } finally {
      setUserLoading(false);
    }
  };

  const handleDeleteUser = async (username: string) => {
    try {
      await aclApi.deleteUser(username);
      message.success(`ACL 用户 ${username} 已删除`);
      fetchUsers();
    } catch {
      // Handled
    }
  };

  const handleToggleUserStatus = async (record: ACLUser, enabled: boolean) => {
    try {
      await aclApi.setUser({
        username: record.username,
        enabled,
        rules: record.rules,
      });
      message.success(`用户 ${record.username} 已${enabled ? '启用' : '禁用'}`);
      fetchUsers();
    } catch {
      // Handled
    }
  };

  const columns = [
    {
      title: '用户名 (User)',
      dataIndex: 'username',
      key: 'username',
      width: 140,
      render: (u: string) => (
        <span style={{ fontWeight: 600, color: u === 'default' ? '#ef4444' : 'inherit' }}>
          {u} {u === 'default' && <Tag color="red">默认用户</Tag>}
        </span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 100,
      render: (en: boolean, record: ACLUser) => (
        <Switch
          checked={en}
          size="small"
          checkedChildren="开启"
          unCheckedChildren="禁用"
          onChange={(checked) => handleToggleUserStatus(record, checked)}
        />
      ),
    },
    {
      title: 'ACL 权限与规则 (Rules)',
      dataIndex: 'rules',
      key: 'rules',
      render: (r: string) => <code>{r || 'allcommands allkeys'}</code>,
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_: any, record: ACLUser) => (
        <Space size="small">
          <Button
            size="small"
            type="link"
            icon={<EditOutlined />}
            onClick={() => {
              setIsEditing(true);
              userForm.setFieldsValue({
                username: record.username,
                enabled: record.enabled,
                rules: record.rules,
                password: '',
              });
              setUserModalVisible(true);
            }}
          >
            编辑/改密
          </Button>
          {record.username !== 'default' && (
            <Popconfirm
              title={`确定删除 ACL 用户 ${record.username}？`}
              onConfirm={() => handleDeleteUser(record.username)}
              okText="删除"
              cancelText="取消"
            >
              <Button size="small" type="link" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* RequirePass Card */}
      <Card
        style={{ borderRadius: '12px', marginBottom: '20px' }}
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <LockOutlined style={{ color: '#ef4444' }} />
            <span>Redis 基础访问密码 (requirepass)</span>
          </div>
        }
        extra={
          <Button
            type="primary"
            icon={<KeyOutlined />}
            onClick={() => {
              setPassInput('');
              setPassModalVisible(true);
            }}
            style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}
          >
            {hasRequirePass ? '修改 / 清空密码' : '设置访问密码'}
          </Button>
        }
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div>
            <Text type="secondary">当前保护状态:</Text>{' '}
            {hasRequirePass ? (
              <Tag color="green" icon={<SafetyOutlined />}>
                已设置密码保护 (requirepass 已开启)
              </Tag>
            ) : (
              <Tag color="warning">未设置密码 (允许无密码访问)</Tag>
            )}
          </div>
          <Text type="secondary" style={{ fontSize: '13px' }}>
            修改后将自动更新 redis.conf 并通过配置保护与探针回滚机制平滑生效。
          </Text>
        </div>
      </Card>

      {/* ACL Users Card */}
      <Card
        style={{ borderRadius: '12px' }}
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <SafetyOutlined style={{ color: '#ef4444' }} />
            <span>Redis 6+ 访问控制列表 (ACL)</span>
          </div>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchUsers}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<UserAddOutlined />}
              onClick={() => {
                setIsEditing(false);
                userForm.resetFields();
                userForm.setFieldsValue({
                  enabled: true,
                  rules: '+@all ~* &*',
                });
                setUserModalVisible(true);
              }}
              style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}
            >
              新建 ACL 用户
            </Button>
          </Space>
        }
      >
        <Alert
          type="info"
          showIcon
          message="基于角色的精细化权限隔离"
          description="Redis 6+ ACL 允许针对不同微服务或开发人员创建专用账号，限制可执行的命令集（如仅读命令 +@read）及允许访问的 Key 前缀模式（如 ~app:*）。"
          style={{ marginBottom: '16px', borderRadius: '8px' }}
        />

        <Table
          columns={columns}
          dataSource={users}
          rowKey="username"
          loading={loading}
          pagination={false}
        />
      </Card>

      {/* Requirepass Modal */}
      <Modal
        title="设置 Redis requirepass 密码"
        open={passModalVisible}
        onCancel={() => setPassModalVisible(false)}
        onOk={handleSaveRequirePass}
        confirmLoading={passLoading}
      >
        <Paragraph type="secondary">
          若需取消密码验证，清空输入框并保存即可。支持一键生成高强度随机密码：
        </Paragraph>
        <Space.Compact style={{ width: '100%', marginBottom: '16px' }}>
          <Input.Password
            placeholder="输入新密码 (留空则清空密码)"
            value={passInput}
            onChange={(e) => setPassInput(e.target.value)}
          />
          <Button icon={<KeyOutlined />} onClick={() => handleGenerateRandomPass('requirepass')}>
            随机强密码
          </Button>
        </Space.Compact>
        <Alert
          type="warning"
          message="密码修改后，所有连接到 Redis 的客户端均需提供 AUTH 认证才能继续读写。"
          showIcon
        />
      </Modal>

      {/* Add / Edit ACL User Modal */}
      <Modal
        title={isEditing ? '编辑 ACL 用户' : '创建新 ACL 用户'}
        open={userModalVisible}
        onCancel={() => setUserModalVisible(false)}
        onOk={() => userForm.submit()}
        confirmLoading={userLoading}
      >
        <Form form={userForm} layout="vertical" onFinish={handleSaveUser}>
          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input disabled={isEditing} placeholder="例如: readonly_app" />
          </Form.Item>

          <Form.Item name="enabled" label="账号状态" valuePropName="checked">
            <Switch checkedChildren="开启 (on)" unCheckedChildren="禁用 (off)" />
          </Form.Item>

          <Form.Item label="密码">
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="password" noStyle>
                <Input.Password placeholder={isEditing ? '留空则保持原密码不变' : '请输入用户密码'} />
              </Form.Item>
              <Button icon={<KeyOutlined />} onClick={() => handleGenerateRandomPass('user')}>
                随机密码
              </Button>
            </Space.Compact>
          </Form.Item>

          <Form.Item
            name="rules"
            label="权限与规则 (ACL Rules)"
            extra="快捷参考: '+@all ~* &*' (全权限), '+@read ~cache:*' (只读cache键), '+@write ~*'"
            rules={[{ required: true, message: '请输入 ACL 规则' }]}
          >
            <Input placeholder="+@all ~* &*" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};
