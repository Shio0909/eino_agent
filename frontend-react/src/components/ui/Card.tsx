import { clsx } from 'clsx';
import type { PropsWithChildren } from 'react';

interface CardProps {
  className?: string;
  tone?: 'panel' | 'flat';
}

export function Card({ className, tone = 'panel', children }: PropsWithChildren<CardProps>) {
  return <section className={clsx('rounded-3xl', tone === 'panel' ? 'glass-panel' : 'border border-border/70 bg-panel/55', className)}>{children}</section>;
}
