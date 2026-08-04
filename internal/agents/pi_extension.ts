// Managed by tap (tmux-agent-panel). `tap uninstall` removes this file;
// local edits will be overwritten by the next `tap install`.
//
// Reports pi's activity into pane-scoped tmux user options (@agent_state,
// @agent_task) so the tap picker can render an agent row for this pane.
// pi has no permission system, so there is no blocked/waiting state.
import type {ExtensionAPI} from '@earendil-works/pi-coding-agent'

const TASK_MAX_LENGTH = 60

export default function (pi: ExtensionAPI) {
	const paneId = process.env.TMUX_PANE
	if (!process.env.TMUX || !paneId) return

	async function safely(run: () => Promise<void>): Promise<void> {
		try {
			await run()
		} catch {}
	}

	async function setOption(name: string, value: string): Promise<void> {
		await safely(async () => {
			await pi.exec('tmux', ['set-option', '-p', '-t', paneId, name, value])
		})
	}

	async function unsetOption(name: string): Promise<void> {
		await safely(async () => {
			await pi.exec('tmux', ['set-option', '-pu', '-t', paneId, name])
		})
	}

	pi.on('session_start', async () => {
		await setOption('@agent_state', 'idle')
	})

	pi.on('before_agent_start', async (event) => {
		await safely(async () => {
			const task = event.prompt
				.replace(/[\t\r\n]+/g, ' ')
				.slice(0, TASK_MAX_LENGTH)
			await setOption('@agent_task', task)
		})
	})

	pi.on('agent_start', async () => {
		await setOption('@agent_state', 'running')
	})

	pi.on('agent_settled', async () => {
		await setOption('@agent_state', 'idle')
	})

	pi.on('session_shutdown', async () => {
		await safely(async () => {
			await unsetOption('@agent_state')
			await unsetOption('@agent_task')
		})
	})
}
