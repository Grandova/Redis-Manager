import React, { useState, useEffect, useRef } from 'react';
import { Row, Col, Card, Radio, Typography, Progress, Space, Tag } from 'antd';
import ReactECharts from 'echarts-for-react';
import {
  DashboardOutlined,
  CloudServerOutlined,
  FieldTimeOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
  SwapOutlined,
  SafetyCertificateOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { MetricCard } from '../../components/MetricCard';
import { StatusBadge } from '../../components/StatusBadge';
import { monitorApi, redisApi } from '../../api';
import { LiveMetrics, HistoryMetrics, RedisDiscovery } from '../../types';

const { Title, Text } = Typography;

export const Dashboard: React.FC = () => {
  const [live, setLive] = useState<LiveMetrics | null>(null);
  const [history, setHistory] = useState<HistoryMetrics | null>(null);
  const [discovery, setDiscovery] = useState<RedisDiscovery | null>(null);
  const [timeRange, setTimeRange] = useState<string>('1h');
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    fetchDiscovery();
    fetchHistory(timeRange);

    // SSE connection for live real-time metrics
    const es = new EventSource('/api/v1/redis/metrics/stream');
    eventSourceRef.current = es;

    es.addEventListener('metrics', (e) => {
      try {
        const data: LiveMetrics = JSON.parse(e.data);
        setLive(data);
      } catch {
        // Ignored
      }
    });

    return () => {
      es.close();
    };
  }, []);

  useEffect(() => {
    fetchHistory(timeRange);
  }, [timeRange]);

  const fetchDiscovery = async () => {
    try {
      const data = await redisApi.getDiscovery();
      setDiscovery(data);
    } catch {
      // Ignored
    }
  };

  const fetchHistory = async (range: string) => {
    try {
      const data = await monitorApi.getHistory(range);
      setHistory(data);
    } catch {
      // Ignored
    }
  };

  const formatBytes = (bytes: number): string => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  // Chart configs
  const getQPSChartOption = () => ({
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: history?.timestamps || [] },
    yAxis: { type: 'value', splitLine: { lineStyle: { type: 'dashed', color: 'rgba(128,128,128,0.15)' } } },
    series: [
      {
        name: 'Commands/sec (QPS)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        itemStyle: { color: '#ef4444' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(239, 68, 68, 0.35)' },
              { offset: 1, color: 'rgba(239, 68, 68, 0.01)' },
            ],
          },
        },
        data: history?.ops || [],
      },
    ],
  });

  const getMemoryChartOption = () => ({
    tooltip: { trigger: 'axis', valueFormatter: (val: any) => `${val} MB` },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: history?.timestamps || [] },
    yAxis: { type: 'value', splitLine: { lineStyle: { type: 'dashed', color: 'rgba(128,128,128,0.15)' } } },
    series: [
      {
        name: 'Redis 内存 (MB)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        itemStyle: { color: '#3b82f6' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(59, 130, 246, 0.3)' },
              { offset: 1, color: 'rgba(59, 130, 246, 0.01)' },
            ],
          },
        },
        data: history?.memory_mb || [],
      },
    ],
  });

  const getNetworkChartOption = () => ({
    tooltip: { trigger: 'axis', valueFormatter: (val: any) => `${val} KB/s` },
    legend: { data: ['入站速度 (In)', '出站速度 (Out)'], top: 0 },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: history?.timestamps || [] },
    yAxis: { type: 'value', splitLine: { lineStyle: { type: 'dashed', color: 'rgba(128,128,128,0.15)' } } },
    series: [
      {
        name: '入站速度 (In)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        itemStyle: { color: '#10b981' },
        data: history?.net_in_kbps || [],
      },
      {
        name: '出站速度 (Out)',
        type: 'line',
        smooth: true,
        showSymbol: false,
        itemStyle: { color: '#f59e0b' },
        data: history?.net_out_kbps || [],
      },
    ],
  });

  const getHitRateChartOption = () => ({
    tooltip: { trigger: 'axis', valueFormatter: (val: any) => `${val}%` },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: history?.timestamps || [] },
    yAxis: { min: 80, max: 100, splitLine: { lineStyle: { type: 'dashed', color: 'rgba(128,128,128,0.15)' } } },
    series: [
      {
        name: '缓存命中率',
        type: 'line',
        smooth: true,
        showSymbol: false,
        itemStyle: { color: '#8b5cf6' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(139, 92, 246, 0.3)' },
              { offset: 1, color: 'rgba(139, 92, 246, 0.01)' },
            ],
          },
        },
        data: history?.hit_rate || [],
      },
    ],
  });

  return (
    <div>
      {/* Top Banner Status */}
      <Card
        style={{
          borderRadius: '12px',
          marginBottom: '20px',
          boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
        }}
        bodyStyle={{ padding: '20px 24px' }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
            <div
              style={{
                width: '48px',
                height: '48px',
                borderRadius: '12px',
                background: 'linear-gradient(135deg, #ef4444 0%, #b91c1c 100%)',
                color: 'white',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '24px',
              }}
            >
              <DatabaseOutlined />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <Title level={4} style={{ margin: 0, fontWeight: 700 }}>
                  {discovery?.version || 'Redis Server'}
                </Title>
                <StatusBadge
                  status={discovery?.is_running ? 'success' : 'error'}
                  text={discovery?.is_running ? '正常运行中' : '未运行'}
                />
                {discovery?.tls_port ? (
                  <Tag color="green" icon={<SafetyCertificateOutlined />}>
                    TLS 已开启
                  </Tag>
                ) : (
                  <Tag color="default">明文 TCP</Tag>
                )}
              </div>
              <div style={{ fontSize: '13px', color: '#888', marginTop: '4px' }}>
                PID: {discovery?.pid || '-'} • 端口: {discovery?.port || 6379} • 模式: {discovery?.mode || 'standalone'} • 开机自启:{' '}
                {discovery?.auto_start ? '已开启' : '已关闭'}
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
            <div>
              <Text type="secondary" style={{ fontSize: '12px' }}>
                累计运行时间
              </Text>
              <div style={{ fontSize: '16px', fontWeight: 600 }}>{discovery?.uptime || '刚刚'}</div>
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: '12px' }}>
                RDB 状态
              </Text>
              <div style={{ fontSize: '16px', fontWeight: 600, color: '#16a34a' }}>
                {live?.rdb_last_bgsave_status === 'ok' ? '正常已持久化' : '未就绪'}
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* KPI Metrics Grid */}
      <Row gutter={[16, 16]} style={{ marginBottom: '20px' }}>
        <Col xs={24} sm={12} md={6}>
          <MetricCard
            title="已用内存 (Used Memory)"
            value={formatBytes(live?.used_memory || 0)}
            subtext={`RSS: ${formatBytes(live?.used_memory_rss || 0)} • 碎片率: ${(live?.mem_frag_ratio || 1).toFixed(2)}`}
            icon={<DashboardOutlined />}
            color="#ef4444"
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <MetricCard
            title="实时 QPS (Commands/s)"
            value={live?.ops_per_sec || 0}
            subtext="每秒处理命令请求数"
            icon={<ThunderboltOutlined />}
            color="#f59e0b"
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <MetricCard
            title="当前在线连接 (Clients)"
            value={live?.connected_clients || 0}
            subtext="活动客户端连接总数"
            icon={<CloudServerOutlined />}
            color="#3b82f6"
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <MetricCard
            title="缓存命中率 (Hit Rate)"
            value={`${(live?.hit_rate || 100).toFixed(1)}%`}
            subtext={`命中: ${live?.keyspace_hits || 0} • 未命中: ${live?.keyspace_misses || 0}`}
            icon={<CheckCircleOutlined />}
            color="#10b981"
          />
        </Col>
      </Row>

      {/* Range Filter */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <Title level={5} style={{ margin: 0 }}>
          性能趋势分析
        </Title>
        <Radio.Group value={timeRange} onChange={(e) => setTimeRange(e.target.value)} buttonStyle="solid">
          <Radio.Button value="1h">最近 1 小时</Radio.Button>
          <Radio.Button value="6h">最近 6 小时</Radio.Button>
          <Radio.Button value="24h">最近 24 小时</Radio.Button>
        </Radio.Group>
      </div>

      {/* Trend Charts */}
      <Row gutter={[16, 16]} style={{ marginBottom: '20px' }}>
        <Col xs={24} lg={12}>
          <Card title="QPS 实时吞吐趋势" bordered style={{ borderRadius: '12px' }}>
            <ReactECharts option={getQPSChartOption()} style={{ height: '260px' }} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="内存消耗趋势 (Memory)" bordered style={{ borderRadius: '12px' }}>
            <ReactECharts option={getMemoryChartOption()} style={{ height: '260px' }} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="网络 I/O 速率 (Network KB/s)" bordered style={{ borderRadius: '12px' }}>
            <ReactECharts option={getNetworkChartOption()} style={{ height: '260px' }} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="缓存命中率趋势 (Hit Rate %)" bordered style={{ borderRadius: '12px' }}>
            <ReactECharts option={getHitRateChartOption()} style={{ height: '260px' }} />
          </Card>
        </Col>
      </Row>
    </div>
  );
};
