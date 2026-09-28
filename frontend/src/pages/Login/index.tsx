import React, { useState } from 'react';
import { Form, Input, Button, Card, Typography, Alert, ConfigProvider, theme } from 'antd';
import { UserOutlined, LockOutlined, DatabaseOutlined, SafetyOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import zhCN from 'antd/locale/zh_CN';
import { authApi } from '../../api';

const { Title, Text } = Typography;

export const Login: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleSubmit = async (values: any) => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await authApi.login(values);
      localStorage.setItem('redis_manager_token', res.token);
      navigate('/dashboard');
    } catch (err: any) {
      setErrorMsg(err.message || '登录失败，请检查用户名或密码');
    } finally {
      setLoading(false);
    }
  };

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#ef4444',
          borderRadius: 8,
          colorBgContainer: '#ffffff',
          colorText: '#0f172a',
          colorBorder: '#cbd5e1',
          fontFamily: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`,
        },
      }}
    >
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: '#f8fafc',
          backgroundImage:
            'radial-gradient(circle at 50% 0%, rgba(239, 68, 68, 0.08) 0%, transparent 55%), radial-gradient(circle at 15% 50%, rgba(241, 245, 249, 0.8) 0%, transparent 50%), radial-gradient(circle at 85% 85%, rgba(226, 232, 240, 0.5) 0%, transparent 50%)',
          padding: '24px 20px',
        }}
      >
        <Card
          style={{
            width: '100%',
            maxWidth: '420px',
            borderRadius: '16px',
            boxShadow: '0 20px 40px -15px rgba(0, 0, 0, 0.07), 0 1px 3px rgba(0, 0, 0, 0.05)',
            border: '1px solid #e2e8f0',
            background: '#ffffff',
          }}
          bodyStyle={{ padding: '40px 36px 36px' }}
        >
          <div style={{ textAlign: 'center', marginBottom: '32px' }}>
            <div
              style={{
                width: '56px',
                height: '56px',
                borderRadius: '14px',
                background: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)',
                color: 'white',
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '28px',
                marginBottom: '16px',
                boxShadow: '0 10px 25px -5px rgba(239, 68, 68, 0.35)',
              }}
            >
              <DatabaseOutlined />
            </div>
            <Title
              level={3}
              style={{
                color: '#0f172a',
                margin: 0,
                fontWeight: 700,
                letterSpacing: '-0.3px',
              }}
            >
              Redis Manager
            </Title>
            <Text style={{ color: '#64748b', fontSize: '13px', display: 'block', marginTop: '6px' }}>
              现代化 Web Redis 运维与数据管理系统
            </Text>
          </div>

          {errorMsg && (
            <Alert
              type="error"
              message={errorMsg}
              showIcon
              style={{
                marginBottom: '20px',
                borderRadius: '8px',
                border: '1px solid #fecaca',
                backgroundColor: '#fef2f2',
              }}
            />
          )}

          <Form layout="vertical" onFinish={handleSubmit} size="large">
            <Form.Item
              name="username"
              rules={[{ required: true, message: '请输入管理员用户名' }]}
              style={{ marginBottom: '18px' }}
            >
              <Input
                prefix={<UserOutlined style={{ color: '#94a3b8' }} />}
                placeholder="管理员用户名 (默认: admin)"
                style={{
                  borderRadius: '8px',
                  height: '44px',
                  backgroundColor: '#f8fafc',
                  borderColor: '#cbd5e1',
                  color: '#0f172a',
                  fontSize: '14px',
                }}
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: '请输入管理员密码' }]}
              style={{ marginBottom: '8px' }}
            >
              <Input.Password
                prefix={<LockOutlined style={{ color: '#94a3b8' }} />}
                placeholder="管理员密码"
                style={{
                  borderRadius: '8px',
                  height: '44px',
                  backgroundColor: '#f8fafc',
                  borderColor: '#cbd5e1',
                  color: '#0f172a',
                  fontSize: '14px',
                }}
              />
            </Form.Item>

            <Form.Item style={{ marginBottom: 0, marginTop: '28px' }}>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                block
                style={{
                  borderRadius: '8px',
                  height: '44px',
                  fontWeight: 600,
                  fontSize: '15px',
                  background: '#ef4444',
                  borderColor: '#ef4444',
                  boxShadow: '0 4px 14px rgba(239, 68, 68, 0.25)',
                }}
              >
                登 录
              </Button>
            </Form.Item>
          </Form>

          <div
            style={{
              marginTop: '28px',
              paddingTop: '20px',
              borderTop: '1px solid #f1f5f9',
              textAlign: 'center',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '6px',
              color: '#94a3b8',
              fontSize: '12px',
            }}
          >
            <SafetyOutlined style={{ color: '#10b981' }} />
            <span>安全会话加密 · 原生 Linux Redis 生产运维</span>
          </div>
        </Card>

        <div style={{ marginTop: '24px', textAlign: 'center', color: '#94a3b8', fontSize: '12px' }}>
          Redis Manager © {new Date().getFullYear()} · Grandova
        </div>
      </div>
    </ConfigProvider>
  );
};

export default Login;
