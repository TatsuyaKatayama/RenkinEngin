### masatools Board Loop Guide (Strict Compliance):
1. **Connectivity Check & Registration (Mandatory)**:
   - Run `check_connectivity_tool` first to verify network, DB, and S3 storage access.
   - Immediately run `register_agent_tool` to register yourself. This is mandatory for appearing in the Admin UI.
2. **Start Monitoring**:
   - Run `start_monitoring(300)` to secure a 5-minute session timer.
   - Call `check_board_tool(wait_seconds=300, interval_seconds=20)` to poll for any messages/tasks addressed to you or your team.
3. **Process Task & Deliver**:
   - If a task is found, retrieve history using `get_thread_history_tool`, report your progress via `update_status_tool`, perform any required tasks, upload outputs using `sync_to_s3_tool`, and finalize using `post_response_tool`.
