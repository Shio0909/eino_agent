import { clsx } from 'clsx';
import type { InputHTMLAttributes } from 'react';

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={clsx('focus-ring w-full rounded-xl border-border/80 bg-panel/80 text-sm text-text placeholder:text-muted', className)} {...props} />;
}
