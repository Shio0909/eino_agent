import { describe, expect, it } from 'vitest';
import { buildLayout } from './GraphPage';
import type { GraphData } from '../types/api';

describe('buildLayout', () => {
  it('keeps dense graph nodes from overlapping', () => {
    const graph: GraphData = {
      nodes: Array.from({ length: 20 }, (_, index) => ({
        id: `node-${index}`,
        label: `Node ${index}`,
        degree: index === 0 ? 50 : 6,
        chunk_count: index === 0 ? 60 : 2,
      })),
      edges: Array.from({ length: 19 }, (_, index) => ({
        source: 'node-0',
        target: `node-${index + 1}`,
        label: 'RELATED',
      })),
    };

    const layout = buildLayout(graph);

    for (let leftIndex = 0; leftIndex < layout.nodes.length; leftIndex += 1) {
      for (let rightIndex = leftIndex + 1; rightIndex < layout.nodes.length; rightIndex += 1) {
        const left = layout.nodes[leftIndex];
        const right = layout.nodes[rightIndex];
        const distance = Math.hypot(left.x - right.x, left.y - right.y);

        expect(distance).toBeGreaterThanOrEqual(48);
      }
    }
  });
});
