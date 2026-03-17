import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center gap-1 rounded-full text-xs font-medium',
  {
    variants: {
      variant: {
        default: 'bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]',
        success: 'bg-green-500/15 text-green-400',
        warning: 'bg-yellow-500/15 text-yellow-400',
        error: 'bg-red-500/15 text-red-400',
        info: 'bg-blue-500/15 text-blue-400',
        accent: 'bg-[var(--color-accent-light)] text-[var(--color-accent)]',
        purple: 'bg-purple-500/15 text-purple-400',
      },
      size: {
        sm: 'px-1.5 py-0.5 text-[10px]',
        md: 'px-2 py-0.5 text-xs',
        lg: 'px-2.5 py-1 text-xs',
      },
    },
    defaultVariants: { variant: 'default', size: 'md' },
  },
)

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, size, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant, size, className }))} {...props} />
}
