import React, { useState, useEffect } from 'react';
import { Layout, Menu, Button, Dropdown, Space, Avatar, Badge, theme, MenuProps } from 'antd';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import {
  DashboardOutlined,
  AppstoreOutlined,
  DatabaseOutlined,
  CodeOutlined,
  TeamOutlined,
  FieldTimeOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  FileTextOutlined,
  HistoryOutlined,
  CloudUploadOutlined,
  SunOutlined,
  MoonOutlined,
  LogoutOutlined,
  UserOutlined,
  LockOutlined,
  SafetyOutlined,
  GlobalOutlined,
} from '@ant-design/icons';
import { authApi, redisApi } from '../api';
import { RedisDiscovery, AdminUser } from '../types';
import { Dashboard } from '../pages/Dashboard';
import { Service } from '../pages/Service';
import { Data } from '../pages/Data';
import { Cli } from '../pages/Cli';
import { Clients } from '../pages/Clients';
import { SlowLog } from '../pages/SlowLog';
import { Config } from '../pages/Config';
import { TLS } from '../pages/TLS';
import { Backup } from '../pages/Backup';
import { Logs } from '../pages/Logs';
import { Audit } from '../pages/Audit';
import { Settings } from '../pages/Settings';
import { ACL } from '../pages/ACL';
import { GeoVPN } from '../pages/GeoVPN';
import './MainLayout.css';

const { Header, Sider, Content } = Layout;

interface MainLayoutProps {
  darkMode: boolean;
  onToggleDarkMode: () => void;
}

export const MainLayout: React.FC<MainLayoutProps> = ({ darkMode, onToggleDarkMode }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();

  const [collapsed, setCollapsed] = useState(false);
  const [currentUser, setCurrentUser] = useState<AdminUser | null>(null);
  const [redisInfo, setRedisInfo] = useState<RedisDiscovery | null>(null);

  // Keep-Alive tab cache: once visited, views stay mounted in DOM to prevent re-fetching and conserve CPU/network
  const [visitedRoutes, setVisitedRoutes] = useState<Set<string>>(() => {
    const init = new Set<string>();
    const current = location.pathname === '/' ? '/dashboard' : location.pathname;
    init.add(current);
    return init;
  });

  useEffect(() => {
    const current = location.pathname === '/' ? '/dashboard' : location.pathname;
    setVisitedRoutes((prev) => {
      if (prev.has(current)) return prev;
      const next = new Set(prev);
      next.add(current);
      return next;
    });
    // Trigger smooth resize for ECharts when returning to cached tabs
    const timer = setTimeout(() => {
      window.dispatchEvent(new Event('resize'));
    }, 80);
    return () => clearTimeout(timer);
  }, [location.pathname]);

  useEffect(() => {
    fetchUser();
    fetchRedisDiscovery();
    const timer = setInterval(fetchRedisDiscovery, 10000);
    return () => clearInterval(timer);
  }, []);

  const fetchUser = async () => {
    try {
      const user = await authApi.current();
      setCurrentUser(user);
    } catch {
      // Ignored
    }
  };

  const fetchRedisDiscovery = async () => {
    try {
      const data = await redisApi.getDiscovery();
      setRedisInfo(data);
    } catch {
      // Ignored
    }
  };

  const handleLogout = async () => {
    try {
      await authApi.logout();
    } finally {
      localStorage.removeItem('redis_manager_token');
      navigate('/login');
    }
  };

  const menuItems: MenuProps['items'] = [
    {
      key: '/dashboard',
      icon: <DashboardOutlined />,
      label: '概览监控',
    },
    {
      key: 'group-redis',
      type: 'group',
      label: 'REDIS 管理',
      children: [
        {
          key: '/service',
          icon: <AppstoreOutlined />,
          label: '服务管理',
        },
        {
          key: '/data',
          icon: <DatabaseOutlined />,
          label: '数据管理',
        },
        {
          key: '/geo-vpn',
          icon: <GlobalOutlined />,
          label: '连接地图',
        },
        {
          key: '/cli',
          icon: <CodeOutlined />,
          label: 'Redis CLI',
        },
        {
          key: '/clients',
          icon: <TeamOutlined />,
          label: '客户端连接',
        },
        {
          key: '/slowlog',
          icon: <FieldTimeOutlined />,
          label: 'Slow Log',
        },
        {
          key: '/backup',
          icon: <CloudUploadOutlined />,
          label: '备份与恢复',
        },
        {
          key: '/logs',
          icon: <FileTextOutlined />,
          label: '实时日志',
        },
      ],
    },
    {
      key: 'group-security',
      type: 'group',
      label: '安全与设置',
      children: [
        {
          key: '/acl',
          icon: <SafetyOutlined />,
          label: 'ACL 权限与密码',
        },
        {
          key: '/tls',
          icon: <SafetyCertificateOutlined />,
          label: 'TLS 自动证书',
        },
        {
          key: '/config',
          icon: <SettingOutlined />,
          label: '配置中心',
        },
      ],
    },
    {
      key: 'group-system',
      type: 'group',
      label: '系统',
      children: [
        {
          key: '/audit',
          icon: <HistoryOutlined />,
          label: '操作审计日志',
        },
        {
          key: '/settings',
          icon: <SettingOutlined />,
          label: '面板设置',
        },
      ],
    },
  ];

  const userMenuItems = [
    {
      key: 'settings',
      icon: <LockOutlined />,
      label: '修改密码',
      onClick: () => navigate('/settings'),
    },
    {
      type: 'divider' as const,
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
      onClick: handleLogout,
    },
  ];

  const getRedisStatusBadge = () => {
    if (!redisInfo) return null;
    if (!redisInfo.is_installed) {
      return (
        <div className="redis-status-pill uninstalled">
          <span className="status-beacon">
            <span className="status-beacon-dot" style={{ backgroundColor: '#ef4444' }} />
          </span>
          <span>未安装 Redis</span>
        </div>
      );
    }
    if (!redisInfo.is_running) {
      return (
        <div className="redis-status-pill stopped">
          <span className="status-beacon">
            <span className="status-beacon-dot" style={{ backgroundColor: '#f59e0b' }} />
          </span>
          <span>已停止 · 端口 {redisInfo.port}</span>
        </div>
      );
    }
    return (
      <div className="redis-status-pill running">
        <span className="status-beacon">
          <span className="status-beacon-ping" style={{ backgroundColor: '#22c55e' }} />
          <span className="status-beacon-dot" style={{ backgroundColor: '#22c55e' }} />
        </span>
        <span style={{ fontWeight: 600 }}>{redisInfo.version ? `Redis ${redisInfo.version}` : 'Redis'}</span>
        <span style={{ opacity: 0.5 }}>·</span>
        <span>运行中</span>
        <span style={{ opacity: 0.85 }}>
          {redisInfo.port === 0
            ? `TLS ${redisInfo.tls_port || 6379} (仅TLS)`
            : `端口 ${redisInfo.port}${redisInfo.tls_port > 0 ? ` / TLS ${redisInfo.tls_port}` : ''}`}
        </span>
      </div>
    );
  };

  return (
    <Layout className={`app-layout ${darkMode ? 'dark-theme' : ''}`}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={240}
        theme={darkMode ? 'dark' : 'light'}
        className="app-sidebar"
        style={{
          background: token.colorBgContainer,
        }}
      >
        <div className="brand-header">
          <div className="brand-logo">
            <DatabaseOutlined style={{ fontSize: '18px' }} />
          </div>
          {!collapsed && (
            <div>
              <div style={{ lineHeight: '1.2' }}>Redis Manager</div>
              <div style={{ fontSize: '10px', color: '#888', fontWeight: 400 }}>Server & Browser</div>
            </div>
          )}
        </div>
        <Menu
          theme={darkMode ? 'dark' : 'light'}
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ borderRight: 0, padding: '8px' }}
        />
      </Sider>

      <Layout>
        <Header
          className="app-header"
          style={{
            background: token.colorBgContainer,
          }}
        >
          <div>{getRedisStatusBadge()}</div>

          <Space size="middle">
            <Button
              type="text"
              shape="circle"
              icon={darkMode ? <SunOutlined /> : <MoonOutlined />}
              onClick={onToggleDarkMode}
              title={darkMode ? '切换为浅色模式' : '切换为深色模式'}
            />

            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" arrow>
              <Space style={{ cursor: 'pointer' }}>
                <Avatar style={{ backgroundColor: '#ef4444' }} icon={<UserOutlined />} />
                <span style={{ fontWeight: 500, fontSize: '13px' }}>
                  {currentUser?.nickname || currentUser?.username || '管理员'}
                </span>
              </Space>
            </Dropdown>
          </Space>
        </Header>

        <Content
          className="app-content"
          style={{
            background: darkMode ? '#141414' : '#f8fafc',
            position: 'relative',
          }}
        >
          {Array.from(visitedRoutes).map((path) => {
            const current = location.pathname === '/' ? '/dashboard' : location.pathname;
            const isActive = current === path;
            let Comp: React.ReactNode = null;
            switch (path) {
              case '/dashboard':
                Comp = <Dashboard />;
                break;
              case '/service':
                Comp = <Service />;
                break;
              case '/data':
                Comp = <Data />;
                break;
              case '/geo-vpn':
                Comp = <GeoVPN />;
                break;
              case '/cli':
                Comp = <Cli />;
                break;
              case '/clients':
                Comp = <Clients />;
                break;
              case '/slowlog':
                Comp = <SlowLog />;
                break;
              case '/config':
                Comp = <Config />;
                break;
              case '/tls':
                Comp = <TLS />;
                break;
              case '/acl':
                Comp = <ACL />;
                break;
              case '/backup':
                Comp = <Backup />;
                break;
              case '/logs':
                Comp = <Logs />;
                break;
              case '/audit':
                Comp = <Audit />;
                break;
              case '/settings':
                Comp = <Settings />;
                break;
              default:
                Comp = null;
            }

            if (!Comp) return null;

            return (
              <div
                key={path}
                style={{
                  display: isActive ? 'block' : 'none',
                  minHeight: '100%',
                }}
              >
                {Comp}
              </div>
            );
          })}
        </Content>
      </Layout>
    </Layout>
  );
};
