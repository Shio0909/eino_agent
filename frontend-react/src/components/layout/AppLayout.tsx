import Sidebar from './Sidebar'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen overflow-hidden bg-[var(--color-bg-primary)] text-[var(--color-text-primary)]">
      <Sidebar />
      <main className="flex-1 overflow-hidden bg-[var(--color-bg-secondary)]/60">
        {children}
      </main>
    </div>
  )
}
