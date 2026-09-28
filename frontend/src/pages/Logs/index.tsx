import React, { useState, useEffect, useRef } from 'react';
import { Card, Input, Select, Button, Space, Switch, Typography, Tag } from 'antd';
import { FileTextOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { redisApi } from '../../api';

export const Logs: React.FC = () => {
  const [logs, setLogs] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [linesCount, setLinesCount] = useState<number>(200);
  const [filterKeyword, setFilterKeyword] = useState<string>('');
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true);

  const logContainerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    fetchLogs();
  }, [linesCount]);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(fetchLogs, 4000);
    return () => clearInterval(timer);
  }, [autoRefresh, linesCount]);

  const fetchLogs = async () => {
    setLoading(true);
    try {
      const res = await redisApi.getLogs(linesCount);
      setLogs(res.lines || []);
    } finally {
      setLoading(false);
    }
  };

  const filteredLogs = logs.filter((line) => {
    if (filterKeyword && !line.toLowerCase().includes(filterKeyword.toLowerCase())) {
      return false;
    }
    if (levelFilter === 'error' && !line.toLowerCase().includes('error') && !line.includes('#')) {
      return false;
    }
    if (levelFilter === 'warning' && !line.toLowerCase().includes('warn')) {
      return false;
    }
    return true;
  });

  const getLineColor = (line: string) => {
    const l = line.toLowerCase();
    if (l.includes('error') || l.includes('failed') || l.includes('fatal')) return '#f87171';
    if (l.includes('warn')) return '#fbbf24';
    if (l.includes('ready') || l.includes('accept connections')) return '#4ade80';
    return '#cbd5e1';
  };

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <FileTextOutlined style={{ color: '#ef4444' }} />
            <span>Redis 实时运行日志</span>
          </div>
        }
        extra={
          <Space>
            <span>自动刷新:</span>
            <Switch checked={autoRefresh} onChange={setAutoRefresh} size="small" />
            <Select
              value={linesCount}
              onChange={setLinesCount}
              size="small"
              style={{ width: 110 }}
              options={[
                { label: '最新 100 行', value: 100 },
                { label: '最新 200 行', value: 200 },
                { label: '最新 500 行', value: 500 },
              ]}
            />
            <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchLogs} size="small">
              刷新
            </Button>
          </Space>
        }
        style={{ borderRadius: '12px' }}
      >
        <div style={{ display: 'flex', gap: '12px', marginBottom: '16px', flexWrap: 'wrap' }}>
          <Input
            placeholder="搜索日志关键词..."
            prefix={<SearchOutlined style={{ color: '#888' }} />}
            value={filterKeyword}
            onChange={(e) => setFilterKeyword(e.target.value)}
            style={{ width: '280px' }}
            allowClear
          />
          <Select
            value={levelFilter}
            onChange={setLevelFilter}
            style={{ width: '130px' }}
            options={[
              { label: '全部级别', value: 'all' },
              { label: 'ERROR (错误)', value: 'error' },
              { label: 'WARNING (告警)', value: 'warning' },
            ]}
          />
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', fontSize: '13px', color: '#666' }}>
            显示 {filteredLogs.length} / {logs.length} 行
          </div>
        </div>

        <div
          ref={logContainerRef}
          style={{
            backgroundColor: '#090d16',
            color: '#e2e8f0',
            fontFamily: 'Consolas, Monaco, "Courier New", monospace',
            fontSize: '12px',
            padding: '16px',
            borderRadius: '8px',
            height: '520px',
            overflowY: 'auto',
            whiteSpace: 'pre-wrap',
            lineHeight: 1.6,
          }}
        >
          {filteredLogs.length > 0 ? (
            filteredLogs.map((line, idx) => (
              <div key={idx} style={{ color: getLineColor(line) }}>
                {line}
              </div>
            ))
          ) : (
            <div style={{ color: '#64748b', textAlign: 'center', paddingTop: '40px' }}>
              暂无匹配的日志记录
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};
