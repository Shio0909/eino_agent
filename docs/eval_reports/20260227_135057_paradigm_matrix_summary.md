# Paradigm Matrix Evaluation Summary

- Time: 2026-02-27 14:32:38
- Base URL: http://localhost:8080
- Datasets: clean(5), large(20)
- Rounds: 3 runs per (dataset, mode)

| dataset | mode | rounds | Recall(avg+/-std) | Precision(avg) | Hit(avg) | MRR(avg) | nDCG(avg) | KW(avg) | P95(avg+/-std, ms) | Error(avg) |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| clean | pipeline | 3 | 0.8 +/- 0 | 0.32 | 0.8 | 0.7 | 0.682 | 0.9667 | 9665.67 +/- 381.07 | 0 |
| clean | agent | 3 | 1 +/- 0 | 0.2 | 1 | 0.725 | 0.7576 | 1 | 14967.67 +/- 812.27 | 0 |
| clean | agentic_rag | 3 | 0.8 +/- 0 | 0.32 | 0.8 | 0.7 | 0.682 | 0.9333 | 9962.33 +/- 78.45 | 0 |
| large | pipeline | 3 | 1 +/- 0 | 0.4 | 1 | 1 | 1 | 0.9083 | 11473.67 +/- 269.77 | 0 |
| large | agent | 3 | 1 +/- 0 | 0.2 | 1 | 1 | 1 | 1 | 20352.33 +/- 2586.33 | 0 |
| large | agentic_rag | 3 | 1 +/- 0 | 0.4 | 1 | 1 | 1 | 0.925 | 11125.33 +/- 64.45 | 0 |

## Notes

- Prefer large dataset numbers in interviews; keep clean set as smoke regression.
- If recall is saturated, show precision/keyword-rate/latency together to avoid one-metric bias.
- Add BEIR subset results for external benchmark comparability.

