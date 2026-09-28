import React, { useState, useRef, useEffect } from 'react';
import { Card, Input, Button, Select, Space, Modal, Typography, Tag } from 'antd';
import { CodeOutlined, ClearOutlined, SendOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import { cliApi } from '../../api';

const { Text } = Typography;

interface HistoryEntry {
  command: string;
  result?: any;
  error?: string;
  timestamp: string;
}

export const Cli: React.FC = () => {
  const [selectedDB, setSelectedDB] = useState<number>(0);
  const [inputCmd, setInputCmd] = useState<string>('');
  const [history, setHistory] = useState<HistoryEntry[]>([
    {
      command: 'INFO',
      result: 'Redis Web CLI 已就绪。支持标准 Redis 交互命令。\n输入 PING, INFO, DBSIZE 等进行测试。',
      timestamp: new Date().toLocaleTimeString(),
    },
  ]);
  const [cmdHistoryList, setCmdHistoryList] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState<number>(-1);
  const [executing, setExecuting] = useState<boolean>(false);

  // Dangerous confirmation modal
  const [dangerModalVisible, setDangerModalVisible] = useState<boolean>(false);
  const [dangerMsg, setDangerMsg] = useState<string>('');
  const [pendingCmd, setPendingCmd] = useState<string>('');

  const terminalEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [history]);

  const handleExecute = async (cmdToExec?: string, force = false) => {
    const cmd = (cmdToExec !== undefined ? cmdToExec : inputCmd).trim();
    if (!cmd) return;

    if (!force) {
      setCmdHistoryList((prev) => [...prev, cmd]);
      setHistoryIndex(-1);
    }

    setExecuting(true);
    setInputCmd('');

    try {
      const res = await cliApi.exec(cmd, selectedDB, force);
      setHistory((prev) => [
        ...prev,
        {
          command: cmd,
          result: res,
          timestamp: new Date().toLocaleTimeString(),
        },
      ]);
    } catch (err: any) {
      if (err.isDangerous) {
        setDangerMsg(err.message || '检测到此操作可能破坏或清除数据');
        setPendingCmd(cmd);
        setDangerModalVisible(true);
      } else {
        setHistory((prev) => [
          ...prev,
          {
            command: cmd,
            error: err.message || '命令执行失败',
            timestamp: new Date().toLocaleTimeString(),
          },
        ]);
      }
    } finally {
      setExecuting(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (cmdHistoryList.length === 0) return;
      const nextIdx = historyIndex === -1 ? cmdHistoryList.length - 1 : Math.max(0, historyIndex - 1);
      setHistoryIndex(nextIdx);
      setInputCmd(cmdHistoryList[nextIdx]);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (historyIndex === -1) return;
      const nextIdx = historyIndex + 1;
      if (nextIdx >= cmdHistoryList.length) {
        setHistoryIndex(-1);
        setInputCmd('');
      } else {
        setHistoryIndex(nextIdx);
        setInputCmd(cmdHistoryList[nextIdx]);
      }
    }
  };

  const handleConfirmDangerous = () => {
    setDangerModalVisible(false);
    handleExecute(pendingCmd, true);
  };

  const renderResult = (res: any) => {
    if (typeof res === 'object') {
      return JSON.stringify(res, null, 2);
    }
    return String(res);
  };

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <CodeOutlined style={{ color: '#ef4444' }} />
            <span>Redis Web CLI 交互终端</span>
            <Tag color="cyan">DB {selectedDB}</Tag>
          </div>
        }
        extra={
          <Space>
            <span>切换数据库:</span>
            <Select
              value={selectedDB}
              onChange={setSelectedDB}
              size="small"
              style={{ width: 90 }}
              options={Array.from({ length: 16 }).map((_, i) => ({
                label: `DB ${i}`,
                value: i,
              }))}
            />
            <Button size="small" icon={<ClearOutlined />} onClick={() => setHistory([])}>
              清屏
            </Button>
          </Space>
        }
        style={{ borderRadius: '12px' }}
        bodyStyle={{ padding: 0 }}
      >
        {/* Terminal Screen */}
        <div
          style={{
            backgroundColor: '#090d16',
            color: '#38bdf8',
            fontFamily: 'Consolas, Monaco, "Courier New", monospace',
            fontSize: '13px',
            padding: '20px',
            minHeight: '480px',
            maxHeight: '600px',
            overflowY: 'auto',
            borderTopLeftRadius: 0,
            borderTopRightRadius: 0,
            lineHeight: 1.6,
          }}
        >
          {history.map((item, idx) => (
            <div key={idx} style={{ marginBottom: '14px' }}>
              <div style={{ color: '#94a3b8', display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ color: '#22c55e' }}>127.0.0.1:6379[{selectedDB}]&gt;</span>
                <span style={{ color: '#f8fafc', fontWeight: 600 }}>{item.command}</span>
                <span style={{ fontSize: '11px', color: '#475569', marginLeft: 'auto' }}>
                  {item.timestamp}
                </span>
              </div>
              {item.error ? (
                <div style={{ color: '#ef4444', marginTop: '4px', whiteSpace: 'pre-wrap' }}>
                  (error) {item.error}
                </div>
              ) : (
                <div style={{ color: '#cbd5e1', marginTop: '4px', whiteSpace: 'pre-wrap' }}>
                  {renderResult(item.result)}
                </div>
              )}
            </div>
          ))}
          <div ref={terminalEndRef} />
        </div>

        {/* Command Input Bar */}
        <div
          style={{
            padding: '12px 16px',
            backgroundColor: '#0f172a',
            borderTop: '1px solid #1e293b',
            display: 'flex',
            gap: '12px',
          }}
        >
          <Input
            value={inputCmd}
            onChange={(e) => setInputCmd(e.target.value)}
            onKeyDown={handleKeyDown}
            onPressEnter={() => handleExecute()}
            placeholder="输入 Redis 命令并回车 (支持 ↑ ↓ 键切换历史命令，高危命令如 FLUSHALL 会被自动拦截)"
            prefix={<span style={{ color: '#22c55e', fontFamily: 'monospace' }}>redis&gt;</span>}
            style={{
              backgroundColor: '#090d16',
              borderColor: '#1e293b',
              color: '#f8fafc',
              fontFamily: 'monospace',
              borderRadius: '6px',
            }}
          />
          <Button
            type="primary"
            icon={<SendOutlined />}
            loading={executing}
            onClick={() => handleExecute()}
            style={{ backgroundColor: '#ef4444', borderColor: '#ef4444', borderRadius: '6px' }}
          >
            执行
          </Button>
        </div>
      </Card>

      {/* Dangerous Confirmation Modal */}
      <Modal
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#ef4444' }} />
            <span>高危命令确认警告</span>
          </Space>
        }
        open={dangerModalVisible}
        onOk={handleConfirmDangerous}
        onCancel={() => setDangerModalVisible(false)}
        okText="确认强制执行"
        okType="danger"
        cancelText="取消"
      >
        <div style={{ padding: '10px 0' }}>
          <Text type="danger" strong style={{ fontSize: '15px' }}>
            {dangerMsg}
          </Text>
          <div style={{ marginTop: '14px', background: '#fef2f2', padding: '12px', borderRadius: '8px', border: '1px solid #fecaca' }}>
            <code>{pendingCmd}</code>
          </div>
          <p style={{ marginTop: '12px', color: '#666', fontSize: '13px' }}>
            高危操作可能导致线上数据永久丢失或 Redis 服务异常停止，请务必核实命令无误！
          </p>
        </div>
      </Modal>
    </div>
  );
};
