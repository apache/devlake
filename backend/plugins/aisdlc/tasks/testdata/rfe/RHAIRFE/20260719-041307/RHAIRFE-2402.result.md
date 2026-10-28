TITLE: Auto ML Test Data Support

| Criterion | Score | Notes |
|-----------|-------|-------|
| WHAT      | 2/2   | Clearly describes the need for advanced Auto ML users to provide their own test datasets instead of relying on automated train/test splits. The problem statement explains the gap (auto-split may not match production scenarios, forcing users to export and test outside the workflow). Acceptance criteria are specific and observable: upload test data, train without auto-split, evaluate with user data, show which dataset was used in UI. |
| WHY       | 2/2   | Names two specific customer accounts: Verizon and Credit Mutuel. Also cites competitive parity with watsonx.ai Auto ML capabilities and references "advanced enterprise customers" requiring production validation workflows. Named customer accounts satisfy the threshold for a score of 2. |
| Open to HOW | 2/2 | All acceptance criteria describe user-facing outcomes and observable behaviors. No internal architecture is prescribed -- no database schemas, no specific implementation approaches, no mandated technologies beyond the existing Auto ML platform vocabulary. Engineering retains full freedom to determine how test data upload, storage, and evaluation are implemented. |
| Not a task | 2/2  | This is a clear business need: enabling advanced users to validate models against production-representative data before deployment to build confidence and reduce deployment risk. It is not a housekeeping chore, tech debt item, or implementation task. |
| Right-sized | 2/2 | All four acceptance criteria are tightly interdependent: providing test data is useless without evaluation using it; evaluation metrics on user data require the data to be provided; the UI indicator requires the test data capability to exist. None can ship independently and provide standalone value. This maps cleanly to a single strategy feature: "user-provided test data support in Auto ML." |
| **Total** | **10/10** | **PASS** |

### Verdict
A well-structured RFE that clearly articulates a specific customer need with named account evidence, leaves implementation to engineering, and is tightly scoped to a single coherent feature.

### Feedback
This RFE is strong across all criteria. Minor improvements that could further strengthen it: (1) quantifying the revenue or deal impact associated with the named customers (e.g., deal size at risk), and (2) specifying any constraints around supported test data formats or size limits if those are known customer requirements. These are polish items rather than structural gaps.
