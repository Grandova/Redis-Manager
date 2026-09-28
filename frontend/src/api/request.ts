import axios from 'axios';
import { message } from 'antd';

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('redis_manager_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

request.interceptors.response.use(
  (response) => {
    const res = response.data;
    // Download file blob check
    if (response.config.responseType === 'blob') {
      return response.data;
    }

    if (res.code === 0) {
      return res.data;
    }

    // High risk CLI code
    if (res.code === 4030) {
      return Promise.reject({ isDangerous: true, message: res.message, data: res.data });
    }

    message.error(res.message || '请求处理失败');
    return Promise.reject(new Error(res.message || 'Error'));
  },
  (error) => {
    if (axios.isCancel(error)) {
      return Promise.reject(error);
    }

    if (error.response) {
      const status = error.response.status;
      const data = error.response.data;

      if (status === 401) {
        localStorage.removeItem('redis_manager_token');
        if (window.location.pathname !== '/login') {
          message.warning('登录已过期，请重新登录');
          window.location.href = '/login';
        }
        return Promise.reject(error);
      }

      const msg = data?.message || `服务器响应错误 (${status})`;
      message.error(msg);
      return Promise.reject(new Error(msg));
    }

    if (error.code === 'ECONNABORTED' || (error.message && error.message.includes('timeout'))) {
      message.error('请求响应超时，请检查后端网络与 Redis 状态');
      return Promise.reject(error);
    }

    message.error(error.message || '网络连接异常，请检查网络');
    return Promise.reject(error);
  }
);

export default request;
