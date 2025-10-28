<script lang="ts">
	import { toScreenCoords } from '$lib/utils';
	import { onMount } from 'svelte';

	let serverAddress = $state('ws://localhost:8080');

	let ws: WebSocket | null = $state(null);
	let connection: RTCPeerConnection | null = $state(null);
	let mouseChannel: RTCDataChannel | null = $state(null);
	let keyboardChannel: RTCDataChannel | null = $state(null);
	let videoElement: HTMLVideoElement | undefined = $state();

	let iceServers: RTCIceServer[] = [];
	let status: string = $state('Websocket connecting...');

	function saveAddress() {
		localStorage.setItem('serverAddress', serverAddress);
	}
	onMount(() => {
		serverAddress = localStorage.getItem('serverAddress') || 'ws://localhost:8080';
	});

	function connect() {
		ws = new WebSocket(`${serverAddress}/ws`);
		ws.onopen = async () => {
			console.log('WebSocket connected');
			status = 'WebSocket connected, waiting for ICE servers...';
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
			if (connection?.connectionState === 'connecting'){
				status = 'WebRTC connecting...';
			}
			if (connection?.connectionState === 'connected') {
				status = '';
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
			1920,
			1080
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
		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			1920,
			1080
		);

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
		const target = event.target as HTMLImageElement;
		const rect = target.getBoundingClientRect();

		const { x, y } = toScreenCoords(
			event.clientX - rect.left,
			event.clientY - rect.top,
			rect.width,
			rect.height,
			1920,
			1080
		);
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
				oncontextmenu={(e) => {
					e.preventDefault();
				}}
				class="h-full w-full"
				bind:this={videoElement}
			>
				<track kind="captions" />
			</video>
			<h1 class="absolute left-1/2 top-1/2 z-10 text-center transform-x-[-50%] transform-y-[-50%]">{status}</h1>
		</div>
	{:else}
		<div class="h-1/2 w-1/2 flex-col rounded bg-neutral-900 p-2.5">
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
					onchange={saveAddress}
					class="w-full rounded bg-neutral-800 p-1"
					placeholder="server address"
				/>

				<button class="rounded bg-neutral-800 p-1" type="submit">Connect</button>
			</form>
		</div>
	{/if}
</div>
