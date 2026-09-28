import React, { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import 'dayjs/locale/zh-cn';

import { MainLayout } from './layouts/MainLayout';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { Service } from './pages/Service';
import { Data } from './pages/Data';
import { Cli } from './pages/Cli';
import { Clients } from './pages/Clients';
import { SlowLog } from './pages/SlowLog';
import { Config } from './pages/Config';
import { TLS } from './pages/TLS';
import { Backup } from './pages/Backup';
import { Logs } from './pages/Logs';
import { Audit } from './pages/Audit';
import { Settings } from './pages/Settings';
import { ACL } from './pages/ACL';
import { GeoVPN } from './pages/GeoVPN';
import './App.css';

// Authentication guard
const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const token = localStorage.getItem('redis_manager_token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
};

export const App: React.FC = () => {
  const [darkMode, setDarkMode] = useState<boolean>(() => {
    return localStorage.getItem('redis_manager_theme') === 'dark';
  });

  const toggleDarkMode = () => {
    setDarkMode((prev) => {
      const next = !prev;
      localStorage.setItem('redis_manager_theme', next ? 'dark' : 'light');
      return next;
    });
  };

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: darkMode ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: '#ef4444',
          borderRadius: 8,
          fontFamily: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`,
        },
      }}
    >
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />

          <Route
            path="/"
            element={
              <ProtectedRoute>
                <MainLayout darkMode={darkMode} onToggleDarkMode={toggleDarkMode} />
              </ProtectedRoute>
            }
          >
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="dashboard" element={<Dashboard />} />
            <Route path="service" element={<Service />} />
            <Route path="data" element={<Data />} />
            <Route path="geo-vpn" element={<GeoVPN />} />
            <Route path="cli" element={<Cli />} />
            <Route path="clients" element={<Clients />} />
            <Route path="slowlog" element={<SlowLog />} />
            <Route path="config" element={<Config />} />
            <Route path="tls" element={<TLS />} />
            <Route path="acl" element={<ACL />} />
            <Route path="backup" element={<Backup />} />
            <Route path="logs" element={<Logs />} />
            <Route path="audit" element={<Audit />} />
            <Route path="settings" element={<Settings />} />
          </Route>

          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
};
export default App;
