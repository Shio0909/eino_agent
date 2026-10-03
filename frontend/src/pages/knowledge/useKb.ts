import { useQuery } from '@tanstack/react-query'
import { apiGet } from '@/lib/api'
import type { KnowledgeBase } from '@/types/knowledge'

/** One knowledge base; shared by the layout and every tab through the query cache. */
export function useKb(kbId: string | undefined) {
  return useQuery({
    queryKey: ['kb', kbId],
    queryFn: () => apiGet<KnowledgeBase>(`/knowledge-bases/${kbId}`),
    enabled: !!kbId,
  })
}
