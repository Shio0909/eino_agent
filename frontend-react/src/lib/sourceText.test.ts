import { describe, expect, it } from 'vitest';
import { isLikelyGarbledSourceText } from './sourceText';

describe('isLikelyGarbledSourceText', () => {
  it('detects PDF extraction mojibake', () => {
    const content = 'B-tree Fe#º² Ñ* … )K`fÄF cYj ^~± ¤ LSM-tree º² R-tree';

    expect(isLikelyGarbledSourceText(content)).toBe(true);
  });

  it('keeps readable Chinese source text', () => {
    const content = 'Go 的垃圾回收器使用三色标记和并发清扫机制，降低应用暂停时间。';

    expect(isLikelyGarbledSourceText(content)).toBe(false);
  });
});
