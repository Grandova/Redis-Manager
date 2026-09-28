import React, { useState, useEffect, useRef } from 'react';
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Tag,
  Button,
  Space,
  Input,
  Select,
  Tooltip,
  Modal,
  Form,
  InputNumber,
  Tabs,
  Badge,
  message,
  Progress,
  Typography,
} from 'antd';
import {
  GlobalOutlined,
  ReloadOutlined,
  SettingOutlined,
  UserOutlined,
  CloudServerOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CopyOutlined,
  CompassOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import * as echarts from 'echarts';
import ReactECharts from 'echarts-for-react';
import chinaGeoJson from '../../assets/china.json';
import { analyticsApi } from '../../api';
import { VPNGeoSummary, ProvinceStat, RegionStat, VPNNodeInfo } from '../../types';

const { Text } = Typography;

// Register China map once
if (!echarts.getMap('china')) {
  echarts.registerMap('china', chinaGeoJson as any);
}

interface ThresholdSettings {
  level1: number; // <= level1: Green
  level2: number; // level1+1 ~ level2: Light Green
  level3: number; // level2+1 ~ level3: Yellow
  level4: number; // level3+1 ~ level4: Orange
  // > level4: Red
}

const DEFAULT_THRESHOLDS: ThresholdSettings = {
  level1: 5,
  level2: 15,
  level3: 30,
  level4: 50,
};

// Module-level in-memory cache for instant 0ms tab switching
let cachedVPNData: VPNGeoSummary | null = null;
let cachedVPNDB = 0;
let cachedVPNPattern = 'soga_conn_*';

export const GeoVPN: React.FC = () => {
  const [dbIdx, setDbIdx] = useState<number>(0);
  const [pattern, setPattern] = useState<string>('soga_conn_*');
  const [loading, setLoading] = useState<boolean>(false);
  const [data, setData] = useState<VPNGeoSummary | null>(() => cachedVPNData);

  // Auto refresh
  const [refreshInterval, setRefreshInterval] = useState<number>(0); // 0 = manual, 5, 10, 30
  const timerRef = useRef<any>(null);

  // Active view tab: china vs overseas
  const [activeGeoTab, setActiveGeoTab] = useState<'china' | 'all'>('china');

  // Selected province filter for nodes table
  const [selectedProvince, setSelectedProvince] = useState<string>('');
  const [nodeSearch, setNodeSearch] = useState<string>('');

  // Threshold settings
  const [thresholdModalVisible, setThresholdModalVisible] = useState<boolean>(false);
  const [thresholds, setThresholds] = useState<ThresholdSettings>(() => {
    try {
      const saved = localStorage.getItem('redis_manager_vpn_thresholds');
      if (saved) return JSON.parse(saved);
    } catch {}
    return DEFAULT_THRESHOLDS;
  });
  const [form] = Form.useForm();

  const fetchData = async (isBackground = false) => {
    if (!isBackground && !data) {
      setLoading(true);
    }
    try {
      const res = await analyticsApi.getVPNGeo(dbIdx, pattern);
      cachedVPNData = res;
      cachedVPNDB = dbIdx;
      cachedVPNPattern = pattern;
      setData(res);
    } catch {
      // Error handled by interceptor
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (cachedVPNData && cachedVPNDB === dbIdx && cachedVPNPattern === pattern) {
      // If we already have fresh cached data, fetch in background silently
      fetchData(true);
    } else {
      fetchData(false);
    }
  }, [dbIdx]);

  useEffect(() => {
    if (refreshInterval > 0) {
      timerRef.current = setInterval(fetchData, refreshInterval * 1000);
    } else if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [refreshInterval, dbIdx, pattern]);

  const copyText = (txt: string) => {
    navigator.clipboard.writeText(txt);
    message.success('已复制: ' + txt);
  };

  const handleSaveThresholds = (values: ThresholdSettings) => {
    setThresholds(values);
    localStorage.setItem('redis_manager_vpn_thresholds', JSON.stringify(values));
    setThresholdModalVisible(false);
    message.success('阈值设置已保存');
  };

  // Build ECharts series data
  const mapData = (data?.provinces || []).map((p) => ({
    name: p.name,
    value: p.count,
    prov: p.prov,
    region: p.region,
    ips: p.ips,
    users: p.users,
    cities: p.cities,
    isps: p.isps,
  }));

  const getMapOption = () => {
    return {
      tooltip: {
        trigger: 'item',
        backgroundColor: 'rgba(255, 255, 255, 0.95)',
        borderColor: '#e2e8f0',
        borderWidth: 1,
        padding: [10, 14],
        textStyle: {
          color: '#1e293b',
          fontSize: 13,
        },
        formatter: (params: any) => {
          if (!params.data) {
            return `<b>${params.name}</b><br/><span style="color:#94a3b8">暂无在线连接</span>`;
          }
          const d = params.data;
          const topCities = Array.from(new Set(d.cities || [])).slice(0, 4).join(', ') || '未知';
          const topISPs = Array.from(new Set(d.isps || [])).slice(0, 3).join(', ') || '未知';
          return `
            <div style="font-weight: 600; font-size: 14px; margin-bottom: 6px; border-bottom: 1px solid #f1f5f9; padding-bottom: 4px;">
              📍 ${d.name} (${d.region})
            </div>
            <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 4px;">
              <span>在线连接数:</span>
              <b style="color: #ef4444; font-size: 15px;">${d.value} 个</b>
            </div>
            <div style="display: flex; justify-content: space-between; gap: 16px; margin-bottom: 4px;">
              <span>独立用户数:</span>
              <b>${d.users?.length || 0} 位</b>
            </div>
            <div style="font-size: 12px; color: #64748b; margin-top: 4px;">
              主要城市: ${topCities}<br/>
              运营商: ${topISPs}
            </div>
          `;
        },
      },
      visualMap: {
        type: 'piecewise',
        orient: 'horizontal',
        left: '20px',
        bottom: '20px',
        pieces: [
          { min: thresholds.level4 + 1, label: `> ${thresholds.level4} 人 (超高)`, color: '#ef4444' },
          { min: thresholds.level3 + 1, max: thresholds.level4, label: `${thresholds.level3 + 1} - ${thresholds.level4} 人 (较高)`, color: '#f97316' },
          { min: thresholds.level2 + 1, max: thresholds.level3, label: `${thresholds.level2 + 1} - ${thresholds.level3} 人 (中等)`, color: '#eab308' },
          { min: thresholds.level1 + 1, max: thresholds.level2, label: `${thresholds.level1 + 1} - ${thresholds.level2} 人 (较低)`, color: '#84cc16' },
          { min: 1, max: thresholds.level1, label: `1 - ${thresholds.level1} 人 (正常)`, color: '#10b981' },
          { value: 0, label: '无连接', color: '#f1f5f9' },
        ],
        textStyle: {
          color: '#475569',
          fontSize: 12,
        },
      },
      geo: {
        map: 'china',
        roam: true,
        zoom: 1.22,
        aspectScale: 0.85,
        layoutCenter: ['50%', '50%'],
        layoutSize: '100%',
        label: {
          show: true,
          fontSize: 10,
          color: '#334155',
        },
        itemStyle: {
          areaColor: '#f8fafc',
          borderColor: '#94a3b8',
          borderWidth: 0.8,
        },
        emphasis: {
          itemStyle: {
            areaColor: '#38bdf8',
            borderColor: '#0284c7',
            borderWidth: 1.5,
          },
          label: {
            show: true,
            color: '#fff',
            fontWeight: 600,
          },
        },
      },
      series: [
        {
          name: '在线用户数',
          type: 'map',
          geoIndex: 0,
          data: mapData,
        },
      ],
    };
  };

  const onChartClick = (params: any) => {
    if (params.name) {
      if (selectedProvince === params.name) {
        setSelectedProvince('');
      } else {
        setSelectedProvince(params.name);
      }
    }
  };

  // Filtered nodes list
  const filteredNodes = (data?.nodes || []).filter((n) => {
    if (selectedProvince && n.province !== selectedProvince) {
      return false;
    }
    if (nodeSearch.trim()) {
      const q = nodeSearch.trim().toLowerCase();
      return (
        n.user_id.toLowerCase().includes(q) ||
        n.ip.toLowerCase().includes(q) ||
        n.prov.toLowerCase().includes(q) ||
        n.city.toLowerCase().includes(q) ||
        n.isp.toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <div>
      {/* Top Header Card */}
      <Card style={{ borderRadius: '12px', marginBottom: '16px' }} bodyStyle={{ padding: '16px 24px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <div
                style={{
                  width: '38px',
                  height: '38px',
                  borderRadius: '8px',
                  backgroundColor: '#fee2e2',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: '#ef4444',
                  fontSize: '20px',
                }}
              >
                <CompassOutlined />
              </div>
              <div>
                <h2 style={{ margin: 0, fontSize: '18px', fontWeight: 700 }}>
                  用户连接地区分布
                </h2>
                <div style={{ fontSize: '12px', color: '#64748b' }}>
                  基于 Redis <code>{pattern}</code> Hash 字段与用户连接 IP 自动定位与实时监控
                </div>
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
            <Space>
              <span style={{ fontSize: '13px', color: '#64748b' }}>选择数据库:</span>
              <Select
                value={dbIdx}
                onChange={setDbIdx}
                style={{ width: '90px' }}
                options={Array.from({ length: 16 }).map((_, i) => ({ label: `DB${i}`, value: i }))}
              />
            </Space>

            <Space>
              <Input
                placeholder="键名通配符 (如: soga_conn_*)"
                value={pattern}
                onChange={(e) => setPattern(e.target.value)}
                onPressEnter={() => fetchData(false)}
                style={{ width: '180px' }}
              />
            </Space>

            <Space>
              <span style={{ fontSize: '13px', color: '#64748b' }}>自动刷新:</span>
              <Select
                value={refreshInterval}
                onChange={setRefreshInterval}
                style={{ width: '100px' }}
                options={[
                  { label: '关闭', value: 0 },
                  { label: '5 秒', value: 5 },
                  { label: '10 秒', value: 10 },
                  { label: '30 秒', value: 30 },
                ]}
              />
            </Space>

            <Button
              icon={<ReloadOutlined spin={loading} />}
              onClick={() => fetchData(false)}
              type="primary"
              style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}
            >
              刷新数据
            </Button>

            <Button
              icon={<SettingOutlined />}
              onClick={() => {
                form.setFieldsValue(thresholds);
                setThresholdModalVisible(true);
              }}
            >
              自定义阈值
            </Button>
          </div>
        </div>

        {/* 4 Stat Cards */}
        <Row gutter={[16, 16]} style={{ marginTop: '20px' }}>
          <Col xs={12} sm={6}>
            <div style={{ padding: '12px 16px', background: '#f8fafc', borderRadius: '8px', border: '1px solid #e2e8f0' }}>
              <div style={{ fontSize: '12px', color: '#64748b', marginBottom: '4px' }}>总在线用户数</div>
              <div style={{ fontSize: '24px', fontWeight: 700, color: '#ef4444' }}>
                {data?.total_users || 0}
                <span style={{ fontSize: '13px', fontWeight: 'normal', color: '#64748b', marginLeft: '4px' }}>位</span>
              </div>
            </div>
          </Col>
          <Col xs={12} sm={6}>
            <div style={{ padding: '12px 16px', background: '#f8fafc', borderRadius: '8px', border: '1px solid #e2e8f0' }}>
              <div style={{ fontSize: '12px', color: '#64748b', marginBottom: '4px' }}>总活跃连接数 (IP)</div>
              <div style={{ fontSize: '24px', fontWeight: 700, color: '#3b82f6' }}>
                {data?.total_connections || 0}
                <span style={{ fontSize: '13px', fontWeight: 'normal', color: '#64748b', marginLeft: '4px' }}>条</span>
              </div>
            </div>
          </Col>
          <Col xs={12} sm={6}>
            <div style={{ padding: '12px 16px', background: '#f8fafc', borderRadius: '8px', border: '1px solid #e2e8f0' }}>
              <div style={{ fontSize: '12px', color: '#64748b', marginBottom: '4px' }}>覆盖中国省份数</div>
              <div style={{ fontSize: '24px', fontWeight: 700, color: '#10b981' }}>
                {data?.total_provinces || 0}
                <span style={{ fontSize: '13px', fontWeight: 'normal', color: '#64748b', marginLeft: '4px' }}>个省市</span>
              </div>
            </div>
          </Col>
          <Col xs={12} sm={6}>
            <div style={{ padding: '12px 16px', background: '#f8fafc', borderRadius: '8px', border: '1px solid #e2e8f0' }}>
              <div style={{ fontSize: '12px', color: '#64748b', marginBottom: '4px' }}>海外 / 局域网</div>
              <div style={{ fontSize: '24px', fontWeight: 700, color: '#8b5cf6' }}>
                {(data?.foreign_count || 0) + (data?.lan_count || 0)}
                <span style={{ fontSize: '13px', fontWeight: 'normal', color: '#64748b', marginLeft: '4px' }}>个</span>
              </div>
            </div>
          </Col>
        </Row>
      </Card>

      {/* Main Two-Column Layout */}
      <Row gutter={[16, 16]}>
        {/* Left: China Map */}
        <Col xs={24} lg={14}>
          <Card
            style={{ borderRadius: '12px', height: '680px', display: 'flex', flexDirection: 'column' }}
            bodyStyle={{ flex: 1, padding: '16px', display: 'flex', flexDirection: 'column', position: 'relative' }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
              <Space>
                <Button
                  type={activeGeoTab === 'china' ? 'primary' : 'default'}
                  onClick={() => setActiveGeoTab('china')}
                  style={activeGeoTab === 'china' ? { backgroundColor: '#ef4444', borderColor: '#ef4444' } : {}}
                >
                  中国地区
                </Button>
                <Button
                  type={activeGeoTab === 'all' ? 'primary' : 'default'}
                  onClick={() => setActiveGeoTab('all')}
                  style={activeGeoTab === 'all' ? { backgroundColor: '#ef4444', borderColor: '#ef4444' } : {}}
                >
                  全部 / 包含海外
                </Button>
                {selectedProvince && (
                  <Tag closable onClose={() => setSelectedProvince('')} color="blue">
                    已选省份: {selectedProvince}
                  </Tag>
                )}
              </Space>

              <div style={{ fontSize: '12px', color: '#94a3b8' }}>
                提示：滚轮可缩放，点击省份可筛选右侧列表
              </div>
            </div>

            <div style={{ flex: 1, width: '100%', minHeight: '560px' }}>
              <ReactECharts
                option={getMapOption()}
                style={{ height: '100%', width: '100%' }}
                onEvents={{ click: onChartClick }}
              />
            </div>
          </Card>
        </Col>

        {/* Right: Regional Breakdown and Node Detail Tabs */}
        <Col xs={24} lg={10}>
          <Card style={{ borderRadius: '12px', height: '680px' }} bodyStyle={{ padding: '12px 16px', height: '100%', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
            <Tabs
              defaultActiveKey="provinces"
              items={[
                {
                  key: 'provinces',
                  label: (
                    <Space>
                      <span>省份连接排名</span>
                      <Badge count={data?.provinces?.length || 0} overflowCount={999} style={{ backgroundColor: '#ef4444' }} />
                    </Space>
                  ),
                  children: (
                    <div style={{ height: '560px', overflowY: 'auto' }}>
                      <Table
                        dataSource={data?.provinces || []}
                        rowKey="name"
                        size="small"
                        pagination={false}
                        columns={[
                          {
                            title: '排名',
                            key: 'rank',
                            width: 50,
                            render: (_: any, __: any, index: number) => (
                              <span
                                style={{
                                  display: 'inline-block',
                                  width: '20px',
                                  height: '20px',
                                  lineHeight: '20px',
                                  textAlign: 'center',
                                  borderRadius: '50%',
                                  background: index === 0 ? '#ef4444' : index === 1 ? '#f97316' : index === 2 ? '#eab308' : '#e2e8f0',
                                  color: index < 3 ? '#fff' : '#475569',
                                  fontWeight: 600,
                                  fontSize: '11px',
                                }}
                              >
                                {index + 1}
                              </span>
                            ),
                          },
                          {
                            title: '省份',
                            dataIndex: 'name',
                            key: 'name',
                            render: (name: string, record: ProvinceStat) => (
                              <span
                                style={{
                                  fontWeight: 600,
                                  cursor: 'pointer',
                                  color: selectedProvince === name ? '#ef4444' : '#1e293b',
                                }}
                                onClick={() => setSelectedProvince(selectedProvince === name ? '' : name)}
                              >
                                {name}
                                <span style={{ marginLeft: '4px', fontSize: '11px', color: '#94a3b8' }}>
                                  ({record.region})
                                </span>
                              </span>
                            ),
                          },
                          {
                            title: '连接数',
                            dataIndex: 'count',
                            key: 'count',
                            width: 80,
                            sorter: (a, b) => a.count - b.count,
                            render: (cnt: number) => <Tag color="blue">{cnt} 人</Tag>,
                          },
                          {
                            title: '占比',
                            dataIndex: 'percent',
                            key: 'percent',
                            width: 120,
                            render: (pct: number) => (
                              <Progress percent={Math.round(pct || 0)} size="small" strokeColor="#ef4444" />
                            ),
                          },
                        ]}
                      />
                    </div>
                  ),
                },
                {
                  key: 'regions',
                  label: '大区统计',
                  children: (
                    <div style={{ height: '560px', overflowY: 'auto' }}>
                      <Table
                        dataSource={data?.regions || []}
                        rowKey="name"
                        size="small"
                        pagination={false}
                        columns={[
                          {
                            title: '地理区域',
                            dataIndex: 'name',
                            key: 'name',
                            width: 110,
                            render: (n: string) => <Tag color="purple" style={{ fontWeight: 600 }}>{n}</Tag>,
                          },
                          {
                            title: '总连接数',
                            dataIndex: 'count',
                            key: 'count',
                            width: 85,
                            render: (c: number) => <b>{c} 个</b>,
                          },
                          {
                            title: '包含活跃省份',
                            key: 'provinces',
                            render: (_: any, r: RegionStat) => (
                              <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                                {r.provinces.map((p) => (
                                  <Tag
                                    key={p.prov}
                                    style={{ cursor: 'pointer' }}
                                    onClick={() => setSelectedProvince(p.name)}
                                  >
                                    {p.prov} ({p.count})
                                  </Tag>
                                ))}
                              </div>
                            ),
                          },
                        ]}
                      />
                    </div>
                  ),
                },
                {
                  key: 'nodes',
                  label: (
                    <Space>
                      <span>在线节点明细</span>
                      <Badge count={filteredNodes.length} overflowCount={999} style={{ backgroundColor: '#3b82f6' }} />
                    </Space>
                  ),
                  children: (
                    <div style={{ height: '560px', display: 'flex', flexDirection: 'column' }}>
                      <div style={{ marginBottom: '8px', display: 'flex', gap: '8px' }}>
                        <Input
                          placeholder="搜索用户 UID、IP 或城市..."
                          prefix={<SearchOutlined style={{ color: '#999' }} />}
                          value={nodeSearch}
                          onChange={(e) => setNodeSearch(e.target.value)}
                          allowClear
                          size="small"
                        />
                      </div>
                      <div style={{ flex: 1, overflowY: 'auto' }}>
                        <Table
                          dataSource={filteredNodes}
                          rowKey={(r) => `${r.user_id}_${r.ip}`}
                          size="small"
                          pagination={false}
                          columns={[
                            {
                              title: '用户 ID',
                              dataIndex: 'user_id',
                              key: 'user_id',
                              width: 100,
                              render: (uid: string) => (
                                <Tag color="volcano" style={{ fontFamily: 'monospace' }}>
                                  UID: {uid}
                                </Tag>
                              ),
                            },
                            {
                              title: '连接 IP',
                              dataIndex: 'ip',
                              key: 'ip',
                              render: (ip: string) => (
                                <Space size={2}>
                                  <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{ip}</span>
                                  <Button
                                    type="text"
                                    size="small"
                                    icon={<CopyOutlined style={{ fontSize: '11px', color: '#94a3b8' }} />}
                                    onClick={() => copyText(ip)}
                                  />
                                </Space>
                              ),
                            },
                            {
                              title: '地区归属',
                              key: 'geo',
                              render: (_: any, r: VPNNodeInfo) => (
                                <div>
                                  <div style={{ fontWeight: 600 }}>{r.prov} {r.city ? `· ${r.city}` : ''}</div>
                                  <div style={{ fontSize: '11px', color: '#94a3b8' }}>{r.isp || '未知网络'}</div>
                                </div>
                              ),
                            },
                            {
                              title: '剩余有效',
                              dataIndex: 'ttl',
                              key: 'ttl',
                              width: 80,
                              render: (ttl: number) => (
                                ttl && ttl > 0 ? (
                                  <Tag color="success">{ttl}s</Tag>
                                ) : (
                                  <Tag color="default">永久</Tag>
                                )
                              ),
                            },
                          ]}
                        />
                      </div>
                    </div>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      {/* Threshold Configuration Modal */}
      <Modal
        title="自定义地图分级阈值设置"
        open={thresholdModalVisible}
        onCancel={() => setThresholdModalVisible(false)}
        onOk={() => form.submit()}
        okText="保存并生效"
        cancelText="取消"
      >
        <div style={{ marginBottom: '16px', color: '#64748b', fontSize: '13px' }}>
          设置各省份连接人数对应的颜色分级区间（数值越大代表连接数越密集）：
        </div>
        <Form form={form} layout="vertical" onFinish={handleSaveThresholds} initialValues={thresholds}>
          <Form.Item
            name="level1"
            label={
              <Space>
                <div style={{ width: '12px', height: '12px', background: '#10b981', borderRadius: '2px' }} />
                <span>绿色分级（正常 / 极低）：1 到此数值</span>
              </Space>
            }
            rules={[{ required: true, message: '请输入阈值' }]}
          >
            <InputNumber min={1} max={10000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            name="level2"
            label={
              <Space>
                <div style={{ width: '12px', height: '12px', background: '#84cc16', borderRadius: '2px' }} />
                <span>浅绿分级（较低）：上一档 + 1 到此数值</span>
              </Space>
            }
            rules={[{ required: true, message: '请输入阈值' }]}
          >
            <InputNumber min={1} max={10000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            name="level3"
            label={
              <Space>
                <div style={{ width: '12px', height: '12px', background: '#eab308', borderRadius: '2px' }} />
                <span>黄色分级（中等）：上一档 + 1 到此数值</span>
              </Space>
            }
            rules={[{ required: true, message: '请输入阈值' }]}
          >
            <InputNumber min={1} max={10000} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            name="level4"
            label={
              <Space>
                <div style={{ width: '12px', height: '12px', background: '#f97316', borderRadius: '2px' }} />
                <span>橙色分级（较高）：上一档 + 1 到此数值（大于此数值显示为红色）</span>
              </Space>
            }
            rules={[{ required: true, message: '请输入阈值' }]}
          >
            <InputNumber min={1} max={10000} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default GeoVPN;
