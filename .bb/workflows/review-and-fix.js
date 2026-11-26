export const meta = {
  name: "review-and-fix",
  description: "Review each completed refactor task, then fix findings",
  phases: [
    { title: "Review", detail: "Independent review by beam-free" },
    { title: "Fix", detail: "Fix findings by deepseek-free" },
  ],
};

const reviewSchema = {
  type: "object",
  required: ["verdict", "findings"],
  properties: {
    verdict: { enum: ["ok", "needs_fix"] },
    findings: {
      type: "array",
      items: {
        type: "object",
        required: ["severity", "summary"],
        properties: {
          severity: { enum: ["critical", "high", "medium", "low"] },
          summary: { type: "string" },
        },
      },
    },
  },
};

const results = await pipeline(
  args.items,
  (item) =>
    agent(
      `You are reviewing completed work in the Go repo at /Users/arturpanteleev/Projects/ai-team (module github.com/arturpanteleev/workaistrator).\n\nTask that was done: ${item.task}\n\nReport from the implementation agent:\n${item.report}\n\nReview the actual current state of the repo (use git status/diff and read the changed files). Check: correctness, that nothing unrelated broke (build/tests), and adherence to the task. Do NOT change any files — review only.\n\nReturn verdict "ok" if the work is acceptable, otherwise "needs_fix" with concrete findings.`,
      {
        label: "Review: " + item.name,
        phase: "Review",
        provider: "acp-opencode",
        model: "opencode/beam-free",
        reasoningLevel: "medium",
        schema: reviewSchema,
      },
    ),
  (review, item) => {
    if (!review || review.verdict === "ok") {
      return { name: item.name, status: "ok", review };
    }
    const findings = review.findings
      .map((f) => `[${f.severity}] ${f.summary}`)
      .join("\n");
    return agent(
      `You are fixing review findings in the Go repo at /Users/arturpanteleev/Projects/ai-team.\n\nOriginal task: ${item.task}\n\nReview findings to fix:\n${findings}\n\nFix ONLY these findings, do not refactor anything else. Run make build and relevant go tests to verify. Do not git commit. Report what you changed and test results.`,
      {
        label: "Fix: " + item.name,
        phase: "Fix",
        provider: "acp-opencode",
        model: "opencode/deepseek-v4.1-flash-free",
        reasoningLevel: "default",
      },
    ).then((fixReport) => ({ name: item.name, status: "fixed", review, fixReport }));
  },
);
return results;
