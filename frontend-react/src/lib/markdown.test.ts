import { describe, expect, it } from 'vitest';
import { renderSafeMarkdown } from './markdown';

describe('renderSafeMarkdown', () => {
  it('escapes raw html before rendering markdown', () => {
    const html = renderSafeMarkdown('# Title\n<script>alert(1)</script> [[Home]]');

    expect(html).toContain('<h1>Title</h1>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('Home');
  });

  it('links source citations in chat markdown', () => {
    const html = renderSafeMarkdown('结论 [来源2]', { linkCitations: true });

    expect(html).toContain('<a href="#source-2">来源2</a>');
  });

  it('maps source ids to numbered citations in chat markdown', () => {
    const html = renderSafeMarkdown('结论 [来源：doc-b]', { linkCitations: true, sourceIds: ['doc-a', 'doc-b'] });

    expect(html).toContain('<a href="#source-2">来源2</a>');
    expect(html).not.toContain('doc-b');
  });
});
