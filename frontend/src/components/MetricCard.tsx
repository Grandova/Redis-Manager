import React from 'react';
import { Card } from 'antd';

interface MetricCardProps {
  title: string;
  value: string | number;
  subtext?: string;
  icon?: React.ReactNode;
  badge?: React.ReactNode;
  trend?: 'up' | 'down' | 'neutral';
  color?: string;
}

export const MetricCard: React.FC<MetricCardProps> = ({
  title,
  value,
  subtext,
  icon,
  badge,
  color,
}) => {
  return (
    <Card
      bordered
      hoverable
      style={{
        borderRadius: '12px',
        height: '100%',
        boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
      }}
      bodyStyle={{ padding: '20px' }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '13px', color: '#888', fontWeight: 500, marginBottom: '6px' }}>
            {title}
          </div>
          <div style={{ fontSize: '26px', fontWeight: 700, letterSpacing: '-0.5px', color: color || 'inherit' }}>
            {value}
          </div>
          {subtext && (
            <div style={{ fontSize: '12px', color: '#999', marginTop: '6px' }}>
              {subtext}
            </div>
          )}
        </div>
        {icon && (
          <div
            style={{
              width: '42px',
              height: '42px',
              borderRadius: '10px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              backgroundColor: 'rgba(239, 68, 68, 0.08)',
              color: '#ef4444',
            }}
          >
            {icon}
          </div>
        )}
      </div>
      {badge && <div style={{ marginTop: '12px' }}>{badge}</div>}
    </Card>
  );
};
