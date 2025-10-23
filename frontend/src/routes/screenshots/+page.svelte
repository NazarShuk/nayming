<script lang="ts">
	import { onMount } from 'svelte';

	let serverAddress = $state('ws://localhost:8080');

	let ws: WebSocket | null = $state(null);
	let connected = $state(false);
	let screenshotData: string | null = $state(null);
	let screenshotWidth: number = $state(0);
	let screenshotHeight: number = $state(0);

	function connect() {
		ws = new WebSocket(`${serverAddress}/ws`);
		ws.onopen = async () => {
			console.log('WebSocket connected');
			connected = true;
		};
		ws.onclose = () => {
			connected = false;
		};
		ws.onerror = (error) => {
			console.log(error);
		};

		ws.onmessage = async (event) => {
			const msg = JSON.parse(event.data);
			console.log(msg);
			if (msg.type === 'screenshot') {
				screenshotData = `data:image/${msg.format};base64,${msg.data}`;
				screenshotHeight = msg.height;
				screenshotWidth = msg.width;
			}
		};
	}

	onMount(() => {
		return () => {
			if (ws) {
				ws.close();
			}
		};
	});

	let canSendMouse = true;
	setInterval(() => {
		canSendMouse = true;
	}, 500);

	function handleMouseMove(event: MouseEvent) {
		event.preventDefault();
		if (!canSendMouse) return;
		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			screenshotWidth,
			screenshotHeight
		);
		ws?.send(
			JSON.stringify({
				type: 'mouse',
				x: x,
				y: y
			})
		);
		canSendMouse = false;
	}
	function handleMouseDown(event: MouseEvent) {
		event.preventDefault();

		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			screenshotWidth,
			screenshotHeight
		);
		ws?.send(
			JSON.stringify({
				type: 'down',
				x: x,
				y: y,
				button: event.button === 0 ? 'left' : 'right'
			})
		);
	}
	function handleMouseUp(event: MouseEvent) {
		event.preventDefault();
		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			screenshotWidth,
			screenshotHeight
		);
		ws?.send(
			JSON.stringify({
				type: 'up',
				x: x,
				y: y,
				button: event.button === 0 ? 'left' : 'right'
			})
		);
	}
	document.addEventListener('keydown', (event) => {
		if (connected) {
			ws?.send(
				JSON.stringify({
					type: 'key',
					key: event.key
				})
			);
		}
	});

	const toScreenCoords = (
		x: number,
		y: number,
		elementWidth: number,
		elementHeight: number,
		width: number,
		height: number
	) => {
		// Calculate the displayed size with object-contain
		const imageAspect = width / height;
		const elementAspect = elementWidth / elementHeight;

		let displayWidth, displayHeight, offsetX, offsetY;

		if (elementAspect > imageAspect) {
			// Element is wider - letterbox on sides
			displayHeight = elementHeight;
			displayWidth = elementHeight * imageAspect;
			offsetX = (elementWidth - displayWidth) / 2;
			offsetY = 0;
		} else {
			// Element is taller - letterbox on top/bottom
			displayWidth = elementWidth;
			displayHeight = elementWidth / imageAspect;
			offsetX = 0;
			offsetY = (elementHeight - displayHeight) / 2;
		}

		// Adjust for letterboxing offset
		const adjustedX = x - offsetX;
		const adjustedY = y - offsetY;

		// Scale to actual screen coordinates
		const scaleX = width / displayWidth;
		const scaleY = height / displayHeight;

		return {
			x: Math.round(adjustedX * scaleX),
			y: Math.round(adjustedY * scaleY)
		};
	};
</script>

<div class="flex h-screen w-full flex-col items-center justify-center bg-neutral-950 text-white">
	{#if connected}
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<img
			onmousemove={handleMouseMove}
			onmousedown={handleMouseDown}
			onmouseup={handleMouseUp}
			oncontextmenu={(e) => {
				e.preventDefault();
			}}
			width={screenshotWidth}
			height={screenshotHeight}
			src={screenshotData}
			alt="screenshot"
			class="h-full w-full object-contain"
		/>
	{:else}
		<div class="h-1/2 w-1/2 rounded bg-neutral-900 p-2.5">
			<h1 class="mb-5 text-xl font-bold">Connect</h1>
			<form
				class="flex flex-row justify-between gap-5"
				onsubmit={(e) => {
					e.preventDefault();
					connect();
				}}
			>
				<input
					bind:value={serverAddress}
					class="w-full rounded bg-neutral-800 p-1"
					placeholder="server address"
				/>
				<button class="rounded bg-neutral-800 p-1" type="submit">Connect</button>
			</form>
			<h2 class="text-red-300">*screenshot only mode*</h2>
		</div>
	{/if}
</div>
