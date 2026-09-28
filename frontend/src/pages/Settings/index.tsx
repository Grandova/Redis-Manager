import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Descriptions, Tag, message, Typography, Row, Col, Divider } from 'antd';
import { SettingOutlined, LockOutlined, DesktopOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { authApi, systemApi } from '../../api';
import { SystemInfo } from '../../types';

const { Title, Paragraph, Text } = Typography;

export const Settings: React.FC = () => {
  const [pwdForm] = Form.useForm();
  const [pwdLoading, setPwdLoading] = useState(false);
  const [sysInfo, setSysInfo] = useState<SystemInfo | null>(null);

  useEffect(() => {
    fetchSystemInfo();
  }, []);

  const fetchSystemInfo = async () => {
    try {
      const data = await systemApi.getInfo();
      setSysInfo(data);
    } catch {
      // Ignored
    }
  };

  const handleUpdatePassword = async (values: any) => {
    setPwdLoading(true);
    try {
      await authApi.updatePassword(values);
      message.success('管理员密码修改成功，请妥善保管！');
      pwdForm.resetFields();
    } catch {
      // Handled
    } finally {
      setPwdLoading(false);
    }
  };

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div>
      <Row gutter={[16, 16]}>
        {/* Password Security */}
        <Col xs={24} md={12}>
          <Card
            title={
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <LockOutlined style={{ color: '#ef4444' }} />
                <span>修改管理员密码</span>
              </div>
            }
            style={{ borderRadius: '12px', height: '100%' }}
          >
            <Paragraph type="secondary" style={{ fontSize: '13px' }}>
              为保障服务器安全，请定期更换管理员访问密码（密码使用 Bcrypt 强散列加密）：
            </Paragraph>

            <Form form={pwdForm} layout="vertical" onFinish={handleUpdatePassword}>
              <Form.Item
                name="old_password"
                label="当前密码"
                rules={[{ required: true, message: '请输入当前密码' }]}
              >
                <Input.Password placeholder="输入原密码" />
              </Form.Item>

              <Form.Item
                name="new_password"
                label="新密码"
                rules={[
                  { required: true, message: '请输入新密码' },
                  { min: 8, message: '密码长度不能少于 8 位' },
                ]}
              >
                <Input.Password placeholder="至少 8 位包含字母数字" />
              </Form.Item>

              <Form.Item style={{ marginTop: '20px' }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={pwdLoading}
                  style={{ backgroundColor: '#ef4444', borderColor: '#ef4444', borderRadius: '6px' }}
                >
                  确认修改密码
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </Col>

        {/* System Host Information */}
        <Col xs={24} md={12}>
          <Card
            title={
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <DesktopOutlined style={{ color: '#ef4444' }} />
                <span>宿主机硬件与环境</span>
              </div>
            }
            style={{ borderRadius: '12px', height: '100%' }}
          >
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="操作系统发行版">
                <Tag color="geekblue">{sysInfo?.platform || sysInfo?.os || '-'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="CPU 核心架构">
                {sysInfo?.cpu_count || 1} 核 ({sysInfo?.arch || 'amd64'}) {sysInfo?.cpu_model ? `• ${sysInfo.cpu_model}` : ''}
              </Descriptions.Item>
              <Descriptions.Item label="物理内存总量">
                {formatBytes(sysInfo?.total_memory || 0)} (空闲: {formatBytes(sysInfo?.free_memory || 0)})
              </Descriptions.Item>
              <Descriptions.Item label="服务器公网 IPv4">
                <code>{sysInfo?.public_ipv4 || '未能自动探测'}</code>
              </Descriptions.Item>
              <Descriptions.Item label="本地内网 IP">
                <code>{sysInfo?.local_ips?.join(', ') || '-'}</code>
              </Descriptions.Item>
              <Descriptions.Item label="主机运行时间">
                {sysInfo ? `${Math.floor(sysInfo.uptime / 86400)} 天 ${Math.floor((sysInfo.uptime % 86400) / 3600)} 小时` : '-'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>

        {/* About Card */}
        <Col xs={24}>
          <Card
            title={
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <InfoCircleOutlined style={{ color: '#ef4444' }} />
                <span>关于 Redis Manager</span>
              </div>
            }
            style={{ borderRadius: '12px' }}
          >
            <Paragraph>
              <b>Redis Manager</b> 是一款现代化、工业级单二进制 Web 原生 Redis 服务运维与数据可视化管理系统。
            </Paragraph>
            <Paragraph type="secondary" style={{ fontSize: '13px' }}>
              版本: v1.0.0 • 架构: React 18 + Vite + Ant Design 5 + Go 1.22+ • 数据库: SQLite Pure Go (CGO-Free)
            </Paragraph>
          </Card>
        </Col>
      </Row>
    </div>
  );
};
