import { clsx } from 'clsx';
import type { PropsWithChildren } from 'react';

interface BadgeProps {
  tone?: 'primary' | 'accent' | 'success' | 'warning' | 'error' | 'muted';
  className?: string;
}

const tones = {
  primary: 'bg-primary/12 text-primary ring-primary/20',
  accent: 'bg-accent/12 text-accent ring-accent/20',
  success: 'bg-success/12 text-success ring-success/20',
  warning: 'bg-warning/12 text-warning ring-warning/20',
  error: 'bg-error/12 text-error ring-error/20',
  muted: 'bg-muted/10 text-muted ring-border/70',
};

export function Badge({ tone = 'muted', className, children }: PropsWithChildren<BadgeProps>) {
  return <span className={clsx('inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold ring-1', tones[tone], className)}>{children}</span>;
}
