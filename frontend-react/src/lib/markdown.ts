import { marked } from 'marked';

const htmlEscapeMap: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
};

interface MarkdownOptions {
  linkCitations?: boolean;
  sourceIds?: string[];
}

export function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (char) => htmlEscapeMap[char]);
}

export function wikiLinkLabel(value: string) {
  return value.replace(/\[\[([^\]]+)\]\]/g, '$1');
}

export function renderSafeMarkdown(value: string, options: MarkdownOptions = {}) {
  const escaped = escapeHtml(wikiLinkLabel(value));
  const withCitations = options.linkCitations ? linkCitations(escaped, options.sourceIds ?? []) : escaped;
  return marked.parse(withCitations, { async: false, breaks: true, gfm: true }) as string;
}

function linkCitations(value: string, sourceIds: string[]) {
  const sourceIndex = new Map(sourceIds.map((id, index) => [id, index + 1]));
  return value
    .replace(/\[来源(\d+)\]/g, (_match, index: string) => `[来源${index}](#source-${index})`)
    .replace(/\[来源[:：]\s*([^\]]+)\]/g, (match, sourceId: string) => {
      const index = sourceIndex.get(sourceId.trim());
      return index ? `[来源${index}](#source-${index})` : match;
    });
}
