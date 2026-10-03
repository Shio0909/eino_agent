import { Hammer } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'

export default function ComingSoon({ title }: { title: string }) {
  return (
    <>
      <PageHeader title={title} />
      <Card>
        <EmptyState icon={<Hammer className="size-5" />} title="这个页面正在重建中" description="新版控制台按页面逐步上线。" />
      </Card>
    </>
  )
}
