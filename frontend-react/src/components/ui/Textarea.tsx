import { clsx } from 'clsx';
import type { TextareaHTMLAttributes } from 'react';

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={clsx('focus-ring w-full resize-none rounded-2xl border-border/80 bg-panel/85 text-sm text-text placeholder:text-muted', className)} {...props} />;
}
