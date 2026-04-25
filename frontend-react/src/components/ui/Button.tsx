import { clsx } from 'clsx';
import type { ButtonHTMLAttributes, PropsWithChildren } from 'react';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
}

const variants = {
  primary: 'bg-primary text-white shadow-soft hover:bg-primary/90',
  secondary: 'bg-text text-surface hover:bg-text/90',
  ghost: 'bg-transparent text-text hover:bg-text/5',
  danger: 'bg-error text-white hover:bg-error/90',
};

export function Button({ className, variant = 'primary', children, ...props }: PropsWithChildren<ButtonProps>) {
  return (
    <button
      className={clsx('focus-ring inline-flex items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50', variants[variant], className)}
      {...props}
    >
      {children}
    </button>
  );
}
