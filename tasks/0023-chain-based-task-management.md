# 0023-chain-based-task-management.md

## Task: Implement Chain-based Task Management

### Purpose
To evolve the task queuing system from managing individual tasks to managing chains of tasks. This change allows for the sequential processing and holistic management of logically connected tasks, improving the orchestration of complex workflows within the `clwclw-monitor` system.

### Description
Currently, the system manages tasks on an individual basis within queues. This enhancement will introduce the concept of a "chain," which is a series of linked tasks. Agents will now claim and process tasks as part of a chain, ensuring that dependent tasks are executed in the correct order and that the overall status of a workflow can be tracked more effectively.

This involves modifications to the Coordinator's task management logic, database schema (to support task chaining), and potentially agent interaction patterns to acknowledge and process tasks within a chain context.

### Requirements from REQUIREMENTS.md
- **Chain 기반 태스크 관리**: 기존 Task 단위로 관리되던 큐잉 시스템을 Chain 단위로 변경한다. 이는 여러 Task가 논리적으로 연결된 경우 이를 하나의 작업 흐름(Chain)으로 간주하여 관리하고 처리하는 것을 의미한다. Chain 내의 Task들은 순차적으로 처리될 수 있으며, Chain 전체의 상태를 추적할 수 있다.

### Checklist
- [x] Update Coordinator API to support chain creation and management.
- [x] Modify database schema to include `chain_id` for tasks and a new `chains` table.
- [x] Implement logic for agents to claim tasks within a specific chain, respecting sequence.
- [x] Update Coordinator's FIFO claim mechanism to prioritize chains or manage task distribution within chains.
- [x] Develop mechanisms to track the overall status of a chain (e.g., `in_progress`, `completed`, `failed`).
- [x] Update webview dashboard to display tasks organized by chains and their statuses.
- [x] Add unit and integration tests for chain-based task management.
- [x] If `ChainID` is not provided during task creation, automatically create a new single-task chain for it.
