<script lang="ts">
	import { toScreenCoords } from '$lib/utils';
	import { onMount } from 'svelte';

	let serverAddress = $state('ws://localhost:8080');
	let password = $state('');

	let ws: WebSocket | null = $state(null);
	let connection: RTCPeerConnection | null = $state(null);
	let mouseChannel: RTCDataChannel | null = $state(null);
	let keyboardChannel: RTCDataChannel | null = $state(null);
	let videoElement: HTMLVideoElement | undefined = $state();

	let iceServers: RTCIceServer[] = [];
	let status: string = $state('Websocket connecting...');

	let screenSize = $state({ width: 1920, height: 1080 });

	function saveAddress() {
		localStorage.setItem('serverAddress', serverAddress);
	}
	function savePassword() {
		localStorage.setItem('password', password);
	}

	onMount(() => {
		serverAddress = localStorage.getItem('serverAddress') || 'ws://localhost:8080';
		password = localStorage.getItem('password') || '';
	});

	let stats = $state({ rtt: 0, jitter: 0 });

	async function updateStats() {
		if (!connection) return;

		const reports = await connection.getStats();
		reports.forEach((report) => {
			if (report.type === 'candidate-pair' && report.state === 'succeeded') {
				stats.rtt = report.currentRoundTripTime * 1000; // Convert to ms
			}
			if (report.type === 'inbound-rtp' && report.kind === 'video') {
				stats.jitter = report.jitter * 1000; // Convert to ms
			}
		});
	}

	onMount(() => {
		const interval = setInterval(updateStats, 1000);
		return () => clearInterval(interval);
	});

	function connect() {
		ws = new WebSocket(`${serverAddress}/ws?password=${password}`);
		ws.onopen = async () => {
			console.log('WebSocket connected');
			status = 'WebSocket connected, waiting for ICE servers...';
		};
		ws.onerror = async () => {
			status = 'Websocket failed to connect. ';
		};

		ws.onmessage = async (event) => {
			const msg = JSON.parse(event.data);

			if (msg.type === 'answer') {
				await connection?.setRemoteDescription({
					type: 'answer',
					sdp: msg.sdp
				});
			} else if (msg.type === 'candidate') {
				const candidate = JSON.parse(msg.candidate);
				await connection?.addIceCandidate(candidate);
			} else if (msg.type === 'iceServers') {
				iceServers = msg.iceServers;
				status = 'ICE servers received, creating connection...';
				console.log('creating connection');
				createConnection();
			} else if (msg.type === 'screenSize') {
				screenSize = msg.screenSize;
			}
		};
	}

	async function createConnection() {
		// Create peer connection
		connection = new RTCPeerConnection({
			iceServers: iceServers
		});

		connection.ontrack = function (event) {
			console.log(event.track.kind);
			if (videoElement) {
				videoElement.srcObject = event.streams[0];
				videoElement.play();
			}
		};

		connection.addTransceiver('video', { direction: 'sendrecv' });

		connection.createDataChannel('alive');
		mouseChannel = connection.createDataChannel('mouse');
		keyboardChannel = connection.createDataChannel('keyboard');

		// Send ICE candidates to server
		connection.onicecandidate = (event) => {
			if (event.candidate) {
				ws?.send(
					JSON.stringify({
						type: 'candidate',
						candidate: JSON.stringify(event.candidate.toJSON())
					})
				);
			}
		};

		// Handle connection state
		connection.onconnectionstatechange = () => {
			console.log('Status: ' + connection?.connectionState);
			if (connection?.connectionState === 'failed') {
				location.reload();
			}
			if (connection?.connectionState === 'connecting') {
				status = 'WebRTC connecting...';
			}
			if (connection?.connectionState === 'connected') {
				status = 'Connected! The screen should appear soon.';
			}
		};

		// Create and send offer
		const offer = await connection.createOffer();
		await connection.setLocalDescription(offer);
		ws?.send(
			JSON.stringify({
				type: 'offer',
				sdp: offer.sdp
			})
		);
	}

	onMount(() => {
		return () => {
			if (ws) {
				ws.close();
			}
			if (connection) {
				connection.close();
			}
			if (keyboardChannel) {
				keyboardChannel.close();
			}
			if (mouseChannel) {
				mouseChannel.close();
			}
		};
	});

	function handleMouseMove(event: MouseEvent) {
		event.preventDefault();
		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			screenSize.width,
			screenSize.height
		);

		if (mouseChannel?.readyState === 'open') {
			mouseChannel?.send(
				JSON.stringify({
					type: 'move',
					x,
					y
				})
			);
		}
	}
	function handleMouseDown(event: MouseEvent) {
		event.preventDefault();

		if (mouseChannel?.readyState === 'open') {
			mouseChannel?.send(
				JSON.stringify({
					type: 'down',
					button: event.button === 0 ? 'left' : 'right'
				})
			);
		}
	}
	function handleMouseUp(event: MouseEvent) {
		event.preventDefault();

		if (mouseChannel?.readyState === 'open') {
			mouseChannel?.send(
				JSON.stringify({
					type: 'up',
					button: event.button === 0 ? 'left' : 'right'
				})
			);
		}
	}
	function handleKeyDown(event: KeyboardEvent) {
		if (!connection) return;
		if (event.repeat) return;
		event.preventDefault();
		if (keyboardChannel?.readyState === 'open') {
			keyboardChannel?.send(
				JSON.stringify({
					type: 'down',
					key: event.key.toLowerCase().replace('arrow', '')
				})
			);
		}
	}
	function handleKeyUp(event: KeyboardEvent) {
		if (!connection) return;
		event.preventDefault();
		if (event.repeat) return;
		if (keyboardChannel?.readyState === 'open') {
			keyboardChannel?.send(
				JSON.stringify({
					type: 'up',
					key: event.key.toLowerCase().replace('arrow', '')
				})
			);
		}
	}
	function handleWheel(event: WheelEvent) {
		if (!connection) return;
		event.preventDefault();
		if (mouseChannel?.readyState === 'open') {
			mouseChannel?.send(
				JSON.stringify({
					type: 'wheel',
					y: -event.deltaY / 100,
					x: -event.deltaX / 100
				})
			);
		}
	}
</script>

<svelte:window onkeydown={handleKeyDown} onkeyup={handleKeyUp} />

<div class="flex h-screen w-full flex-col items-center justify-center bg-neutral-950 text-white">
	{#if ws}
		<div class="relative flex h-full w-full">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<video
				onmousemove={handleMouseMove}
				onmousedown={handleMouseDown}
				onmouseup={handleMouseUp}
				onwheel={handleWheel}
				oncontextmenu={(e) => {
					e.preventDefault();
				}}
				class="h-full w-full"
				bind:this={videoElement}
				onplaying={() => (status = '')}
			>
				<track kind="captions" />
			</video>
			<h1
				style="transform: translate(-50%, -50%);"
				class="absolute top-1/2 left-1/2 z-10 text-center"
			>
				{status}
			</h1>

			<div
				class="absolute top-4 left-4 z-20 rounded bg-black/50 p-2 font-mono text-xs text-green-400"
			>
				<div>RTT: {stats.rtt.toFixed(0)}ms</div>
				<div>Jitter: {stats.jitter.toFixed(2)}ms</div>
			</div>
		</div>
	{:else}
		<div class="h-fit w-[95%] flex-col rounded bg-neutral-900 p-2.5 md:w-1/2">
			<h1 class="mb-5 text-xl font-bold">Nayming</h1>
			<form
				class="flex flex-row justify-between gap-5"
				onsubmit={(e) => {
					e.preventDefault();
					connect();
				}}
			>
				<div class="flex w-full flex-col gap-2.5">
					<input
						bind:value={serverAddress}
						onchange={saveAddress}
						class="w-full rounded bg-neutral-800 p-1"
						placeholder="server address"
					/>
					<input
						bind:value={password}
						onchange={savePassword}
						class="w-full rounded bg-neutral-800 p-1"
						placeholder="password"
						type="password"
					/>
				</div>

				<button class="m-auto h-fit rounded bg-neutral-800 p-1" type="submit">Connect</button>
			</form>
		</div>
	{/if}
</div>
