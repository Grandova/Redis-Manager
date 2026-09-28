import React from 'react';

interface StatusBadgeProps {
  status: 'success' | 'error' | 'warning' | 'default' | 'processing';
  text: string;
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, text }) => {
  const getColor = () => {
    switch (status) {
      case 'success':
        return { bg: 'rgba(34, 197, 94, 0.12)', dot: '#22c55e', text: '#15803d' };
      case 'error':
        return { bg: 'rgba(239, 68, 68, 0.12)', dot: '#ef4444', text: '#b91c1c' };
      case 'warning':
        return { bg: 'rgba(245, 158, 11, 0.12)', dot: '#f59e0b', text: '#b45309' };
      case 'processing':
        return { bg: 'rgba(59, 130, 246, 0.12)', dot: '#3b82f6', text: '#1d4ed8' };
      default:
        return { bg: 'rgba(156, 163, 175, 0.12)', dot: '#9ca3af', text: '#4b5563' };
    }
  };

  const style = getColor();

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        padding: '3px 10px',
        borderRadius: '9999px',
        fontSize: '12px',
        fontWeight: 500,
        backgroundColor: style.bg,
        color: style.text,
      }}
    >
      <span
        style={{
          width: '6px',
          height: '6px',
          borderRadius: '50%',
          backgroundColor: style.dot,
          marginRight: '6px',
        }}
      />
      {text}
    </span>
  );
};
