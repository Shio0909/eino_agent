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
});
