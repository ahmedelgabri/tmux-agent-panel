-- Lists every terminal block in every Rex session in one `rex do` round trip,
-- with the fields tmux's list-panes format gives the picker. A script hears
-- only the sessions it attaches to, so it attaches to each before reading it.
local terminal = 'com.superlogical.terminal'

local function call(method, args)
	return rex.block.call(terminal, method, args) or {}
end

local panes = {}
for _, session in ipairs(rex.call('session.list', {}).sessions) do
	rex.session.attach { session_id = session.session_id }
	local view = rex.call('session.view', { session_id = session.session_id })
	for window_index, window in ipairs(view and view.windows or {}) do
		local pane_index = 0
		for _, layer in ipairs(window.layers or {}) do
			for _, block in ipairs(layer.blocks or {}) do
				if block.creator_name == terminal then
					pane_index = pane_index + 1
					local target = {
						session_id = session.session_id,
						block_id = block.block_id,
					}
					local foreground = call('process', target).foreground or {}
					local root = {}
					for _, record in ipairs(call('program_status', target).records or {}) do
						if record.id == nil then
							root = record
						end
					end
					table.insert(panes, {
						block_id = block.block_id,
						session = session.label or session.session_id,
						window = window.label or '',
						window_index = window_index,
						pane_index = pane_index,
						command = foreground.name,
						path = foreground.cwd,
						pane_title = call('title', target).title,
						state = root.state,
						kind = root.kind,
						app = root.app,
						msg = root.msg,
						title = root.title,
					})
				end
			end
		end
	end
end
return panes
