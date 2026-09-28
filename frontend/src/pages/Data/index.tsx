import React, { useState, useEffect } from 'react';
import {
  Card,
  Tabs,
  Input,
  Button,
  Select,
  Table,
  Tag,
  Space,
  Modal,
  Drawer,
  Form,
  InputNumber,
  message,
  Popconfirm,
  Badge,
  Typography,
  Tooltip,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import {
  SearchOutlined,
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  SaveOutlined,
  GlobalOutlined,
  CopyOutlined,
  FieldTimeOutlined,
} from '@ant-design/icons';
import { dataApi } from '../../api';
import { KeyItem, KeyDetail, DBStat } from '../../types';

const { Text, Paragraph } = Typography;

// Module-level in-memory cache for instant 0ms tab switching
let cachedKeysList: KeyItem[] = [];
let cachedCursor = 0;
let cachedDB = 0;
let cachedDBStats: DBStat[] = [];
let cachedPattern = '*';
let cachedKeyType = '';

export const Data: React.FC = () => {
  const navigate = useNavigate();

  const copyText = (txt: string) => {
    navigator.clipboard.writeText(txt);
    message.success('已复制到剪贴板');
  };

  const [selectedDB, setSelectedDB] = useState<number>(cachedDB);
  const [dbStats, setDBStats] = useState<DBStat[]>(cachedDBStats);
  const [keys, setKeys] = useState<KeyItem[]>(cachedKeysList);
  const [cursor, setCursor] = useState<number>(cachedCursor);
  const [pattern, setPattern] = useState<string>(cachedPattern);
  const [keyType, setKeyType] = useState<string>(cachedKeyType);
  const [loading, setLoading] = useState<boolean>(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  // Key detail drawer
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [activeKeyDetail, setActiveKeyDetail] = useState<KeyDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // String edit state
  const [stringVal, setStringVal] = useState<string>('');

  // Add Key Modal
  const [addModalVisible, setAddModalVisible] = useState(false);
  const [addForm] = Form.useForm();

  // Rename modal
  const [renameModalVisible, setRenameModalVisible] = useState(false);
  const [targetRenameKey, setTargetRenameKey] = useState<string>('');
  const [newKeyName, setNewKeyName] = useState<string>('');

  // TTL modal
  const [ttlModalVisible, setTtlModalVisible] = useState(false);
  const [targetTTLKey, setTargetTTLKey] = useState<string>('');
  const [newTTLVal, setNewTTLVal] = useState<number>(-1);

  // Auto sync with machine (default 5s)
  const [autoSyncInterval, setAutoSyncInterval] = useState<number>(5);

  // 1. Live dynamic 1-second countdown clock for TTL
  useEffect(() => {
    const timer = setInterval(() => {
      setKeys((prevKeys) => {
        if (!prevKeys || prevKeys.length === 0) return prevKeys;
        return prevKeys.map((item) => {
          let updatedTTL = item.ttl;
          if (item.ttl > 0) {
            updatedTTL = Math.max(0, item.ttl - 1);
          }
          let updatedFieldTTL = item.field_ttl;
          if (item.field_ttl && item.field_ttl > 0) {
            updatedFieldTTL = Math.max(0, item.field_ttl - 1);
          }
          let updatedFields = item.fields;
          if (item.fields && item.fields.length > 0) {
            updatedFields = item.fields.map((f) => ({
              ...f,
              ttl: f.ttl && f.ttl > 0 ? Math.max(0, f.ttl - 1) : f.ttl,
            }));
          }
          return {
            ...item,
            ttl: updatedTTL,
            field_ttl: updatedFieldTTL,
            fields: updatedFields,
          };
        });
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // 2. Periodic silent auto-sync with machine
  useEffect(() => {
    if (autoSyncInterval <= 0) return;
    const timer = setInterval(() => {
      scanKeys(selectedDB, 0, pattern, keyType, true);
    }, autoSyncInterval * 1000);
    return () => clearInterval(timer);
  }, [autoSyncInterval, selectedDB, pattern, keyType]);

  useEffect(() => {
    fetchDBStats();
    if (cachedKeysList.length > 0 && cachedDB === selectedDB && cachedPattern === pattern) {
      scanKeys(selectedDB, 0, pattern, keyType, true);
    } else {
      scanKeys(selectedDB, 0, pattern, keyType, false);
    }
  }, [selectedDB]);

  const fetchDBStats = async () => {
    try {
      const stats = await dataApi.getDBStats();
      setDBStats(stats);
      cachedDBStats = stats;
    } catch {
      // Ignored
    }
  };

  const scanKeys = async (db: number, cur: number, pat: string, type: string, silent = false) => {
    if (!silent && keys.length === 0) {
      setLoading(true);
    }
    try {
      const res = await dataApi.scanKeys(db, cur, pat, 50, type);
      const newKeys = cur === 0 ? (res.keys || []) : [...keys, ...(res.keys || [])];
      setKeys(newKeys);
      setCursor(res.cursor || 0);
      cachedKeysList = newKeys;
      cachedCursor = res.cursor || 0;
      cachedDB = db;
      cachedPattern = pat;
      cachedKeyType = type;
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = () => {
    scanKeys(selectedDB, 0, pattern, keyType, false);
  };

  const handleViewKey = async (key: string) => {
    setDrawerVisible(true);
    setDetailLoading(true);
    try {
      const detail = await dataApi.getKey(selectedDB, key);
      setActiveKeyDetail(detail);
      if (detail.type === 'string') {
        setStringVal(typeof detail.value === 'string' ? detail.value : JSON.stringify(detail.value));
      }
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDeleteKey = async (key: string) => {
    try {
      await dataApi.deleteKey(selectedDB, key);
      message.success(`Key '${key}' 已删除`);
      scanKeys(selectedDB, 0, pattern, keyType);
      fetchDBStats();
      if (activeKeyDetail?.key === key) {
        setDrawerVisible(false);
      }
    } catch {
      // Handled
    }
  };

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) return;
    try {
      await dataApi.batchDeleteKeys(selectedDB, selectedRowKeys as string[]);
      message.success(`成功批量删除 ${selectedRowKeys.length} 个键`);
      setSelectedRowKeys([]);
      scanKeys(selectedDB, 0, pattern, keyType);
      fetchDBStats();
    } catch {
      // Handled
    }
  };

  const handleSaveStringValue = async () => {
    if (!activeKeyDetail) return;
    try {
      await dataApi.saveKey(selectedDB, {
        key: activeKeyDetail.key,
        type: 'string',
        ttl: activeKeyDetail.ttl,
        value: stringVal,
      });
      message.success('保存成功');
      handleViewKey(activeKeyDetail.key);
    } catch {
      // Handled
    }
  };

  const handleAddKey = async (values: any) => {
    try {
      await dataApi.saveKey(selectedDB, values);
      message.success(`Key '${values.key}' 创建成功`);
      setAddModalVisible(false);
      addForm.resetFields();
      scanKeys(selectedDB, 0, pattern, keyType);
      fetchDBStats();
    } catch {
      // Handled
    }
  };

  const handleRename = async () => {
    if (!newKeyName.trim()) {
      message.error('请输入新键名');
      return;
    }
    try {
      await dataApi.renameKey(selectedDB, targetRenameKey, newKeyName.trim());
      message.success('重命名成功');
      setRenameModalVisible(false);
      scanKeys(selectedDB, 0, pattern, keyType);
    } catch {
      // Handled
    }
  };

  const handleSetTTL = async () => {
    try {
      await dataApi.setTTL(selectedDB, targetTTLKey, newTTLVal);
      message.success('TTL 设置成功');
      setTtlModalVisible(false);
      scanKeys(selectedDB, 0, pattern, keyType);
    } catch {
      // Handled
    }
  };

  const getTypeTag = (type: string) => {
    switch (type) {
      case 'string':
        return <Tag color="green">String</Tag>;
      case 'hash':
        return <Tag color="purple">Hash</Tag>;
      case 'list':
        return <Tag color="blue">List</Tag>;
      case 'set':
        return <Tag color="orange">Set</Tag>;
      case 'zset':
        return <Tag color="cyan">ZSet</Tag>;
      case 'stream':
        return <Tag color="magenta">Stream</Tag>;
      default:
        return <Tag>{type}</Tag>;
    }
  };

  const columns = [
    {
      title: '键名 (Key)',
      dataIndex: 'key',
      key: 'key',
      width: 220,
      render: (text: string) => (
        <Space size={4}>
          <span
            style={{ fontWeight: 600, cursor: 'pointer', color: '#ef4444' }}
            onClick={() => handleViewKey(text)}
          >
            {text}
          </span>
          <Tooltip title="复制键名">
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined style={{ fontSize: '12px', color: '#999' }} />}
              onClick={(e) => {
                e.stopPropagation();
                copyText(text);
              }}
            />
          </Tooltip>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 90,
      render: (t: string, record: KeyItem) => (
        <Space direction="vertical" size={2}>
          {getTypeTag(t)}
          {record.length !== undefined && record.length > 0 && (
            <span style={{ fontSize: '11px', color: '#999' }}>{record.length} 项</span>
          )}
        </Space>
      ),
    },
    {
      title: '键值预览 / 字段',
      key: 'value_preview',
      render: (_: any, record: KeyItem) => {
        if (record.type === 'hash') {
          if (record.fields && record.fields.length > 0) {
            return (
              <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', alignItems: 'center' }}>
                {record.fields.map((f, idx) => (
                  <Tooltip
                    key={idx}
                    title={
                      <div>
                        <div>字段 (Field/IP): <b>{f.field}</b></div>
                        <div>值 (Value): {f.value}</div>
                        {f.ttl && f.ttl > 0 ? (
                          <div style={{ color: '#52c41a' }}>字段剩余有效时间: {f.ttl} 秒</div>
                        ) : null}
                      </div>
                    }
                  >
                    <Tag
                      color="blue"
                      style={{ cursor: 'pointer', margin: '2px 4px 2px 0' }}
                      onClick={(e) => {
                        e.stopPropagation();
                        copyText(f.field);
                      }}
                    >
                      <GlobalOutlined style={{ marginRight: '4px' }} />
                      {f.field}
                      {f.ttl && f.ttl > 0 && (
                        <span style={{ marginLeft: '4px', opacity: 0.85, fontSize: '11px' }}>({f.ttl}s)</span>
                      )}
                    </Tag>
                  </Tooltip>
                ))}
                {record.length && record.length > record.fields.length ? (
                  <span style={{ fontSize: '12px', color: '#999' }}>+{record.length - record.fields.length} 个字段...</span>
                ) : null}
              </div>
            );
          }
          return <span style={{ color: '#888' }}>{record.value_preview || '-'}</span>;
        }

        if (record.type === 'string') {
          return (
            <Tooltip title="点击复制内容">
              <span
                onClick={(e) => {
                  e.stopPropagation();
                  copyText(record.value_preview || '');
                }}
                style={{
                  fontFamily: 'monospace',
                  fontSize: '12px',
                  background: '#f8fafc',
                  padding: '2px 6px',
                  borderRadius: '4px',
                  border: '1px solid #e2e8f0',
                  cursor: 'pointer',
                  maxWidth: '380px',
                  display: 'inline-block',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {record.value_preview || '<空字符串>'}
              </span>
            </Tooltip>
          );
        }

        return <span style={{ fontFamily: 'monospace', fontSize: '12px', color: '#4b5563' }}>{record.value_preview || '-'}</span>;
      },
    },
    {
      title: 'TTL (过期时间)',
      dataIndex: 'ttl',
      key: 'ttl',
      width: 170,
      render: (ttl: number, record: KeyItem) => {
        // Prioritize field-level TTL if hash fields have expiration (e.g. Soga 60s timeout)
        if (record.field_ttl && record.field_ttl > 0) {
          return (
            <Tooltip title={`该 Hash 字段设置了过期倒计时 (如 60s 超时)，最新活跃字段剩余有效时间: ${record.field_ttl} 秒`}>
              <Tag color="cyan" icon={<FieldTimeOutlined />} style={{ fontWeight: 600 }}>
                字段 TTL: {record.field_ttl} 秒
              </Tag>
            </Tooltip>
          );
        }
        if (ttl > 0) {
          return <Tag color="warning" icon={<ClockCircleOutlined />}>{ttl} 秒</Tag>;
        }
        if (ttl === -1) return <Tag color="default">永久不过期</Tag>;
        if (ttl === -2) return <Tag color="error">已失效</Tag>;
        return <Tag color="warning">{ttl} 秒</Tag>;
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_: any, record: KeyItem) => (
        <Space size="small">
          <Button size="small" type="link" onClick={() => handleViewKey(record.key)}>
            查看/编辑
          </Button>
          <Button
            size="small"
            type="link"
            onClick={() => {
              setTargetTTLKey(record.key);
              setNewTTLVal(record.ttl);
              setTtlModalVisible(true);
            }}
          >
            TTL
          </Button>
          <Button
            size="small"
            type="link"
            onClick={() => {
              setTargetRenameKey(record.key);
              setNewKeyName(record.key);
              setRenameModalVisible(true);
            }}
          >
            重命名
          </Button>
          <Popconfirm
            title="确认删除该键？"
            onConfirm={() => handleDeleteKey(record.key)}
            okText="删除"
            cancelText="取消"
          >
            <Button size="small" type="link" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* DB Switcher Tabs */}
      <Card style={{ borderRadius: '12px', marginBottom: '16px' }} bodyStyle={{ padding: '8px 16px' }}>
        <Tabs
          activeKey={String(selectedDB)}
          onChange={(k) => setSelectedDB(Number(k))}
          items={Array.from({ length: 16 }).map((_, i) => {
            const count = dbStats[i]?.keys || 0;
            return {
              key: String(i),
              label: (
                <Space>
                  <span>DB{i}</span>
                  {count > 0 && <Badge count={count} overflowCount={99999} style={{ backgroundColor: '#ef4444' }} />}
                </Space>
              ),
            };
          })}
        />
      </Card>

      {/* Toolbar & Search */}
      <Card style={{ borderRadius: '12px', marginBottom: '16px' }}>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
          <Input
            placeholder="搜索 Key (支持通配符: user:*)"
            prefix={<SearchOutlined style={{ color: '#999' }} />}
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            onPressEnter={handleSearch}
            style={{ width: '260px' }}
          />
          <Select
            placeholder="类型筛选"
            value={keyType}
            onChange={setKeyType}
            style={{ width: '120px' }}
            allowClear
            options={[
              { label: '全部类型', value: '' },
              { label: 'String', value: 'string' },
              { label: 'Hash', value: 'hash' },
              { label: 'List', value: 'list' },
              { label: 'Set', value: 'set' },
              { label: 'ZSet', value: 'zset' },
              { label: 'Stream', value: 'stream' },
            ]}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch} style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}>
            SCAN 搜索
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => scanKeys(selectedDB, 0, pattern, keyType, false)}>
            刷新
          </Button>

          <Space>
            <span style={{ fontSize: '13px', color: '#64748b' }}>自动同步机器:</span>
            <Select
              value={autoSyncInterval}
              onChange={setAutoSyncInterval}
              style={{ width: '95px' }}
              options={[
                { label: '关闭', value: 0 },
                { label: '3 秒', value: 3 },
                { label: '5 秒', value: 5 },
                { label: '10 秒', value: 10 },
                { label: '30 秒', value: 30 },
              ]}
            />
          </Space>

          <Button
            type="primary"
            icon={<GlobalOutlined />}
            onClick={() => navigate('/geo-vpn')}
            style={{ backgroundColor: '#10b981', borderColor: '#10b981' }}
          >
            连接地图
          </Button>

          <div style={{ marginLeft: 'auto', display: 'flex', gap: '8px' }}>
            {selectedRowKeys.length > 0 && (
              <Popconfirm
                title={`确定批量删除选中的 ${selectedRowKeys.length} 个键？`}
                onConfirm={handleBatchDelete}
                okText="删除"
                cancelText="取消"
              >
                <Button danger icon={<DeleteOutlined />}>
                  批量删除 ({selectedRowKeys.length})
                </Button>
              </Popconfirm>
            )}
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddModalVisible(true)}>
              新增 Key
            </Button>
          </div>
        </div>
      </Card>

      {/* Keys Table */}
      <Card style={{ borderRadius: '12px' }}>
        <Table
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
          }}
          columns={columns}
          dataSource={keys}
          rowKey="key"
          loading={loading}
          pagination={false}
          expandable={{
            expandedRowRender: (record: KeyItem) => {
              if (record.type === 'hash' && record.fields && record.fields.length > 0) {
                return (
                  <div style={{ padding: '8px 16px', background: '#fafafa', borderRadius: '8px' }}>
                    <div style={{ marginBottom: '8px', fontWeight: 600, fontSize: '13px', color: '#475569' }}>
                      Hash 内部字段详情 (共 {record.length || record.fields.length} 个字段):
                    </div>
                    <Table
                      dataSource={record.fields}
                      rowKey="field"
                      pagination={false}
                      size="small"
                      columns={[
                        {
                          title: 'Field (键 / 用户 IP)',
                          dataIndex: 'field',
                          key: 'field',
                          render: (f: string) => (
                            <Space>
                              <Tag color="blue"><GlobalOutlined /> {f}</Tag>
                              <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => copyText(f)} />
                            </Space>
                          ),
                        },
                        {
                          title: 'Value (值 / 状态)',
                          dataIndex: 'value',
                          key: 'value',
                          render: (v: string) => <span style={{ fontFamily: 'monospace' }}>{v}</span>,
                        },
                        {
                          title: '字段剩余 TTL',
                          dataIndex: 'ttl',
                          key: 'ttl',
                          render: (t?: number) => t && t > 0 ? (
                            <Tag color="success"><FieldTimeOutlined /> {t} 秒</Tag>
                          ) : (
                            <Tag color="default">跟随主键/无单独过期</Tag>
                          ),
                        },
                      ]}
                    />
                  </div>
                );
              }
              if (record.type === 'string') {
                return (
                  <div style={{ padding: '12px', background: '#fafafa', borderRadius: '8px' }}>
                    <div style={{ marginBottom: '6px', fontWeight: 600, fontSize: '13px', color: '#475569' }}>完整字符串内容:</div>
                    <pre style={{ margin: 0, padding: '8px 12px', background: '#fff', border: '1px solid #e2e8f0', borderRadius: '6px', maxHeight: '180px', overflow: 'auto', fontFamily: 'monospace', fontSize: '12px' }}>
                      {record.value_preview || ''}
                    </pre>
                  </div>
                );
              }
              return (
                <div style={{ padding: '8px', color: '#666' }}>
                  预览内容: {record.value_preview || '无内容'}
                </div>
              );
            },
          }}
        />
        {cursor > 0 && (
          <div style={{ textAlign: 'center', marginTop: '16px' }}>
            <Button
              type="dashed"
              onClick={() => scanKeys(selectedDB, cursor, pattern, keyType)}
              loading={loading}
            >
              加载下一页 (SCAN Cursor: {cursor})
            </Button>
          </div>
        )}
      </Card>

      {/* Key Detail Drawer */}
      <Drawer
        title={
          <Space>
            <span>{activeKeyDetail?.key}</span>
            {activeKeyDetail && getTypeTag(activeKeyDetail.type)}
          </Space>
        }
        width={650}
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        extra={
          activeKeyDetail?.type === 'string' && (
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveStringValue} style={{ backgroundColor: '#ef4444', borderColor: '#ef4444' }}>
              保存变更
            </Button>
          )
        }
      >
        {detailLoading ? (
          <div>加载中...</div>
        ) : activeKeyDetail ? (
          <div>
            <div style={{ marginBottom: '16px', display: 'flex', gap: '16px', flexWrap: 'wrap', alignItems: 'center', fontSize: '13px', color: '#666' }}>
              <div>主键 TTL: {activeKeyDetail.ttl === -1 ? '永久不过期' : `${activeKeyDetail.ttl} 秒`}</div>
              {activeKeyDetail.field_ttl && activeKeyDetail.field_ttl > 0 ? (
                <div style={{ color: '#0ea5e9', fontWeight: 600 }}>
                  <FieldTimeOutlined style={{ marginRight: '4px' }} />
                  Hash 字段 TTL: 剩余 {activeKeyDetail.field_ttl} 秒
                </div>
              ) : null}
              <div>长度/字段数: {activeKeyDetail.length || '-'}</div>
            </div>

            {/* String View/Edit */}
            {activeKeyDetail.type === 'string' && (
              <div>
                <Input.TextArea
                  rows={16}
                  value={stringVal}
                  onChange={(e) => setStringVal(e.target.value)}
                  style={{ fontFamily: 'monospace', fontSize: '13px' }}
                />
              </div>
            )}

            {/* Hash View */}
            {activeKeyDetail.type === 'hash' && (
              <Table
                dataSource={Object.entries(activeKeyDetail.value || {}).map(([f, v]) => ({
                  field: f,
                  value: String(v),
                  ttl: activeKeyDetail.field_ttls ? activeKeyDetail.field_ttls[f] : undefined,
                }))}
                rowKey="field"
                size="small"
                columns={[
                  {
                    title: 'Field (键 / 用户 IP)',
                    dataIndex: 'field',
                    width: 220,
                    render: (f: string) => (
                      <Space>
                        <Tag color="blue"><GlobalOutlined /> {f}</Tag>
                        <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => copyText(f)} />
                      </Space>
                    ),
                  },
                  {
                    title: 'Value (值)',
                    dataIndex: 'value',
                    render: (v: string) => <span style={{ fontFamily: 'monospace' }}>{v}</span>,
                  },
                  {
                    title: '字段剩余有效 TTL',
                    dataIndex: 'ttl',
                    width: 160,
                    render: (t?: number) =>
                      t && t > 0 ? (
                        <Tag color="success"><FieldTimeOutlined /> {t} 秒</Tag>
                      ) : (
                        <Tag color="default">跟随主键/无单独过期</Tag>
                      ),
                  },
                ]}
              />
            )}

            {/* List View */}
            {activeKeyDetail.type === 'list' && (
              <Table
                dataSource={(Array.isArray(activeKeyDetail.value) ? activeKeyDetail.value : []).map((val, idx) => ({ index: idx, value: String(val) }))}
                rowKey="index"
                size="small"
                columns={[
                  { title: 'Index', dataIndex: 'index', width: 80 },
                  { title: 'Value', dataIndex: 'value' },
                ]}
              />
            )}

            {/* Set View */}
            {activeKeyDetail.type === 'set' && (
              <Table
                dataSource={(Array.isArray(activeKeyDetail.value) ? activeKeyDetail.value : []).map((val, idx) => ({ index: idx, member: String(val) }))}
                rowKey="index"
                size="small"
                columns={[{ title: 'Member 成员', dataIndex: 'member' }]}
              />
            )}

            {/* ZSet View */}
            {activeKeyDetail.type === 'zset' && (
              <Table
                dataSource={(Array.isArray(activeKeyDetail.value) ? activeKeyDetail.value : []).map((item: any, idx: number) => ({ index: idx, member: item.Member, score: item.Score }))}
                rowKey="index"
                size="small"
                columns={[
                  { title: 'Member 成员', dataIndex: 'member' },
                  { title: 'Score 分数', dataIndex: 'score', width: 120 },
                ]}
              />
            )}

            {/* Stream View */}
            {activeKeyDetail.type === 'stream' && (
              <div style={{ maxHeight: '500px', overflowY: 'auto' }}>
                <pre style={{ background: '#f8fafc', padding: '12px', borderRadius: '8px' }}>
                  {JSON.stringify(activeKeyDetail.value, null, 2)}
                </pre>
              </div>
            )}
          </div>
        ) : null}
      </Drawer>

      {/* Add Key Modal */}
      <Modal
        title="创建新 Key"
        open={addModalVisible}
        onCancel={() => setAddModalVisible(false)}
        onOk={() => addForm.submit()}
      >
        <Form form={addForm} layout="vertical" onFinish={handleAddKey} initialValues={{ type: 'string', ttl: -1 }}>
          <Form.Item name="key" label="键名 (Key)" rules={[{ required: true, message: '请输入键名' }]}>
            <Input placeholder="例如: user:1001:profile" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'String (字符串)', value: 'string' },
                { label: 'Hash (哈希散列)', value: 'hash' },
                { label: 'List (列表)', value: 'list' },
                { label: 'Set (集合)', value: 'set' },
                { label: 'ZSet (有序集合)', value: 'zset' },
              ]}
            />
          </Form.Item>
          <Form.Item name="ttl" label="TTL 过期秒数 (-1 为永久)">
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.type !== cur.type}>
            {({ getFieldValue }) => {
              const type = getFieldValue('type');
              if (type === 'hash') {
                return (
                  <>
                    <Form.Item name="field" label="Field (散列字段名)" rules={[{ required: true }]}>
                      <Input placeholder="例如: username" />
                    </Form.Item>
                    <Form.Item name="value" label="Value (值)" rules={[{ required: true }]}>
                      <Input.TextArea rows={3} />
                    </Form.Item>
                  </>
                );
              }
              if (type === 'zset') {
                return (
                  <>
                    <Form.Item name="score" label="Score (分数)" rules={[{ required: true }]}>
                      <InputNumber style={{ width: '100%' }} defaultValue={0} />
                    </Form.Item>
                    <Form.Item name="value" label="Member (成员)" rules={[{ required: true }]}>
                      <Input placeholder="成员值" />
                    </Form.Item>
                  </>
                );
              }
              return (
                <Form.Item name="value" label="Value (值)" rules={[{ required: true }]}>
                  <Input.TextArea rows={4} placeholder="写入的数据内容" />
                </Form.Item>
              );
            }}
          </Form.Item>
        </Form>
      </Modal>

      {/* Rename Modal */}
      <Modal
        title={`重命名 Key: ${targetRenameKey}`}
        open={renameModalVisible}
        onCancel={() => setRenameModalVisible(false)}
        onOk={handleRename}
      >
        <Input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="输入新的键名" />
      </Modal>

      {/* TTL Modal */}
      <Modal
        title={`设置 TTL: ${targetTTLKey}`}
        open={ttlModalVisible}
        onCancel={() => setTtlModalVisible(false)}
        onOk={handleSetTTL}
      >
        <Paragraph>输入过期的秒数，输入 -1 则移除过期时间设为永久：</Paragraph>
        <InputNumber style={{ width: '100%' }} value={newTTLVal} onChange={(v) => setNewTTLVal(v || -1)} />
      </Modal>
    </div>
  );
};
