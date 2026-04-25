import { marked } from 'marked';

const htmlEscapeMap: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
};

export function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (char) => htmlEscapeMap[char]);
}

export function wikiLinkLabel(value: string) {
  return value.replace(/\[\[([^\]]+)\]\]/g, '$1');
}

export function renderSafeMarkdown(value: string) {
  const escaped = escapeHtml(wikiLinkLabel(value));
  return marked.parse(escaped, { async: false, breaks: true, gfm: true }) as string;
}
