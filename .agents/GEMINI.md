# Automatic Superpowers Skill Routing & Activation

Any AI agent operating in this workspace must automatically evaluate every user prompt against the **Superpowers Framework** skills before taking action or writing code:

1. **Automatic Skill Selection**:
   - **Feature Creation / UI / Enhancements**: Automatically activate `brainstorming` -> `writing-plans` -> `executing-plans`.
   - **Bug Fixes / Test Failures / Diagnostic Errors**: Automatically activate `systematic-debugging` -> `test-driven-development`.
   - **Multi-Task / Complex Architecture**: Automatically activate `dispatching-parallel-agents` or `subagent-driven-development`.
   - **Pre-Merge / Verification**: Automatically activate `verification-before-completion` and `requesting-code-review`.

2. **Zero User Overhead**:
   - The user NEVER needs to specify which skill or subagent to use.
   - The agent MUST autonomously choose the exact skill that fits the task.
