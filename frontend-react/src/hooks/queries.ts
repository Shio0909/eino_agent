import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { endpoints } from './endpoints';

export function useKnowledgeBases() {
  return useQuery({ queryKey: ['knowledge-bases'], queryFn: endpoints.knowledgeBases });
}

export function useCreateKnowledgeBase() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: endpoints.createKnowledgeBase,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['knowledge-bases'] }),
  });
}

export function useDocuments(kbId?: string | null) {
  return useQuery({
    queryKey: ['documents', kbId],
    queryFn: () => endpoints.documents(kbId!),
    enabled: Boolean(kbId),
    refetchInterval: 5000,
  });
}

export function useWikiPages(kbId?: string | null) {
  return useQuery({
    queryKey: ['wiki-pages', kbId],
    queryFn: () => endpoints.wikiPages(kbId!),
    enabled: Boolean(kbId),
  });
}

export function useWikiPage(kbId?: string | null, path?: string | null) {
  return useQuery({
    queryKey: ['wiki-page', kbId, path],
    queryFn: () => endpoints.wikiPage(kbId!, path!),
    enabled: Boolean(kbId && path),
  });
}

export function useSessions() {
  return useQuery({ queryKey: ['sessions'], queryFn: endpoints.sessions });
}

export function useGraph(kbId?: string | null, limit = 200) {
  return useQuery({
    queryKey: ['graphrag-graph', kbId, limit],
    queryFn: () => endpoints.graph(kbId!, limit),
    enabled: Boolean(kbId),
  });
}

export function useOpsStatus() {
  return {
    mcp: useQuery({ queryKey: ['mcp'], queryFn: endpoints.mcp }),
    settings: useQuery({ queryKey: ['settings'], queryFn: endpoints.settings }),
    system: useQuery({ queryKey: ['system-info'], queryFn: endpoints.systemInfo }),
    graph: useQuery({ queryKey: ['graphrag-status'], queryFn: endpoints.graphStatus }),
  };
}
